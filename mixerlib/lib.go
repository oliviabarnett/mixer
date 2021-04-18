package mixerlib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	jobcoin "github.com/oliviabarnett/mixer"
	"github.com/oliviabarnett/mixer/clientlib"
	"github.com/oliviabarnett/mixer/internal"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type MixerConfig struct {
	registeredTransactions *internal.RegisteredTransactions
	houseAddress string
	api clientlib.API
	dispatcher *internal.Dispatcher
	collectedFees float64
}

// ServeMixer spins up an endpoint to which mixing requests are sent. Also creates the dispatcher which
// will handle adding and executing mixing jobs.
func ServeMixer() {
	mixerConfig := initializeMixer()
	rand.Seed(time.Now().UnixNano())

	r := mux.NewRouter()
	r.HandleFunc("/send", mixerConfig.sendHandler)
	http.Handle("/", r)

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start Server and await mixing requests
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// Start job dispatcher which will send out final transactions
	mixerConfig.dispatcher.Start(10)

	waitForShutdown(srv)
	fmt.Printf("Fee Collection Total: %f \n", mixerConfig.collectedFees)
	mixerConfig.dispatcher.Finished()
}

// initializeMixer sets up the config required to run a mixing service
func initializeMixer() *MixerConfig {
	return &MixerConfig{
		houseAddress:           uuid.NewString(),
		registeredTransactions: internal.NewRegisteredTransactions(),
		dispatcher:             internal.NewDispatcher(),
		collectedFees:          0,
		api:                   	&clientlib.JobCoinAPI{Client: http.DefaultClient},
	}
}

// pollAddress watches the address that the user promised to send JobCoin to for a certain amount time.
// If JobCoin hasn't been sent in that window, abandon the job.
func (mixerConfig *MixerConfig) pollAddress(address string) {
	startTime := time.Now()
	ticker := time.NewTicker(time.Second * 5)

	// Each address being polled will have its own set of "processedTransactions" to ensure we don't repeat send
	processedTransactions := internal.NewSet()

	defer ticker.Stop()
	done := make(chan bool)
	go func() {
		//  If a transaction has not been detected after a set amount of time, abandon scanning this deposit address.
		time.Sleep(jobcoin.TransactionWindow * time.Second)
		done <- true
	}()
	for {
		select {
		case <-done:
			// When abandoning the deposit address delete it from the set of registeredTransactions.
			mixerConfig.registeredTransactions.Delete(address)
			return
		case <-ticker.C:
			addressInfo, err := mixerConfig.api.GetAddressInfo(address)
			if err != nil {
				continue
			}
			mixerConfig.processTransactions(addressInfo.Transactions, processedTransactions, startTime)
		}
	}
}

// processTransactions identifies transactions that need processing and moves the transaction amount from the
// deposit address to the house address then kicks off mixing to the destination addresses
func (mixerConfig *MixerConfig) processTransactions(transactions []internal.Transaction, processedTransactions *internal.Set, startTime time.Time) {
	for _, transaction := range transactions {
		if processedTransactions.Contains(transaction.AsSha256()) || transaction.Timestamp.Before(startTime) {
			continue
		}
		destinationAddresses, ok := mixerConfig.registeredTransactions.Load(transaction.ToAddress)
		if !ok {
			continue
		}
		fmt.Printf("processing: %v \n", transaction)

		// Record the time at which the transaction was recorded so we know when to execute the transaction
		transactionStartTime := time.Now()

		// If the transaction is new, has been posted after we started listening, and is to an active deposit address
		// then we will proceed with processing it. First step is to send the amount to the house address.
		_, err := mixerConfig.api.SendCoin(transaction.ToAddress, mixerConfig.houseAddress, transaction.Amount)

		// If sending to the house address fails, another polling pass will attempt to send again.
		if err != nil {
			continue
		}

		// Mix this amount and dole it out to destinationAddresses
		mixed := mixerConfig.mixToAddresses(destinationAddresses, transaction.Amount, transactionStartTime)

		// Record that we processed the transaction
		if mixed {
			processedTransactions.Add(transaction.AsSha256())
		}
	}
}

// distribute (pseudo) randomly divides an amount over the number of destination addresses and returns the
// distribution as an array. It also assigns an execution time within the configured duration and returns this
// as an array. Finally, distribute also collects and returns the fee.
func distribute(destinationAddresses []string, amount string, startTime int64) (float64, []float64, []time.Time, error) {
	if len(destinationAddresses) < 1 {
		return 0.0, nil, nil, errors.New("no destination addresses to distribute over")
	}
	numBuckets := len(destinationAddresses)

	amounts := make([]float64, numBuckets)
	partitions := make([]float64, numBuckets)
	times := make([]time.Time, numBuckets)

	balance, err := strconv.ParseFloat(amount, 64)

	fee := balance * jobcoin.Fee
	balance -= fee

	// Generate (pseudo) random times within a configured time interval
	for i := range destinationAddresses {
		max := startTime + jobcoin.DepositInterval
		delta := max - startTime
		randomTime := rand.Int63n(delta) + startTime

		times[i] = time.Unix(randomTime, 0)
	}

	// Generate (pseudo) random partitions to distribute the amount over the addresses
	// One thing to note here is that it is possible to end up with an amount of 0.
	sum := 0.0
	for j := 0; j < numBuckets; j++ {
		randomNum := rand.Float64() * balance
		partitions[j] = randomNum
		sum = sum + randomNum
	}
	sort.Float64s(partitions)

	for k, p := range partitions {
		frac := p/sum
		amounts[k] = frac*balance
	}

	return fee, amounts, times, err
}

// mixToAddresses determines the random amounts and intervals for the mixed transactions and then dispatches
// jobs to send the final transactions to the jobCoin api.
func (mixerConfig *MixerConfig) mixToAddresses(destinationAddresses []string, amount string, startTime time.Time) bool {
	feeCollected, amounts, times, err := distribute(destinationAddresses, amount, startTime.Unix())
	mixerConfig.collectedFees += feeCollected

	if err != nil || len(amounts) != len(destinationAddresses) {
		return false
	}
	fmt.Printf("amounts: %v \n", amounts)

	// Current implementation distributes (random) amounts over the destination addresses -- one transaction per
	// address given. However, there could be _more_ than one transaction per address destination which might be
	// more secure. Good future feature.
	for i, address := range destinationAddresses {
		go func(destinationAddress string, amountToSend float64, executionTime time.Time) {
			fmt.Printf("Create job for %s \n", destinationAddress)
			job := func() error {
				sendingAmount := fmt.Sprintf("%f", amountToSend)
				fmt.Printf("Sending %s from %s to %s \n", sendingAmount, mixerConfig.houseAddress, destinationAddress)
				status, err := mixerConfig.api.SendCoin(mixerConfig.houseAddress, destinationAddress, sendingAmount)
				fmt.Printf("%s from %s with error %s \n", status, destinationAddress, err)
				return err
			}
			mixerConfig.dispatcher.AddJob(destinationAddress, job, executionTime)
		}(address, amounts[i], times[i])
	}
	return true
}

// sendHandler awaits input specifying addresses to which JobCoin should be sent and the agreed upon deposit address.
// The desired addresses are saved under the deposit address and a listener is spun up targeting the deposit adress
// and waiting for transactions to mix.
func (mixerConfig *MixerConfig) sendHandler(w http.ResponseWriter, r *http.Request) {
	var receivedData map[string]string
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&receivedData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(r.Body)

	addresses, ok := receivedData["addresses"]
	if !ok { return }

	depositAddress, ok := receivedData["deposit"]
	if !ok { return }

	// Save new transactions
	// What if you want to send to A and B but we already have those registered...
	// Hashmap won't work in this case. Another reason to use LRU cache?
	_, ok = mixerConfig.registeredTransactions.Load(depositAddress)
	if !ok {
		mixerConfig.registeredTransactions.Store(depositAddress, strings.Split(strings.ToLower(addresses), ","))

		go func() {
			mixerConfig.pollAddress(depositAddress)
		}()
	}

	respondWithJSON(w, http.StatusCreated, "Received transaction")
}

// waitForShutdown handles the shutting down of the mixer service.
func waitForShutdown(srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-interruptChan
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_ = srv.Shutdown(ctx)

	log.Printf("\n Shutting down mixer... \n")
	os.Exit(0)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}