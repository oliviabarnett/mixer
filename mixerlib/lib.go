package mixerlib

import (
	"context"
	"encoding/json"
	"fmt"
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
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Could this be an LRU cache?
// When a transaction is registered, that means we are actively searching for it.
// Deposit address --> list of addresses


type MixerConfig struct {
	registeredTransactions *internal.RegisteredTransactions
	houseAddress string
	api clientlib.API
	dispatcher *internal.Dispatcher
	collectedFees float64
}

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

	// Start job dispatcher which will send out transactions
	mixerConfig.dispatcher.Start(10)

	waitForShutdown(srv)
	fmt.Printf("Fee Collection Total: %f \n", mixerConfig.collectedFees)
	mixerConfig.dispatcher.Finished()
}

func initializeMixer() *MixerConfig {
	return &MixerConfig{
		houseAddress:           "House address", // uuid.NewString() // TODO: make UUID when done testing
		registeredTransactions: internal.NewRegisteredTransactions(),
		dispatcher:             internal.NewDispatcher(),
		collectedFees:          0,
		api:                   	&clientlib.JobCoinAPI{Client: http.DefaultClient},
	}
}

// PollAddress watches the address that the user promised to send JobCoin to for a certain amount time.
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

// ProcessTransactions identifies transactions that need processing and moves the transaction amount from the
// deposit address to the house address then kicks off mixing to the destination addresses
func (mixerConfig *MixerConfig) processTransactions(transactions []internal.Transaction, processedTransactions *internal.Set, startTime time.Time) {
	for _, transaction := range transactions {
		fmt.Printf("processing: %v \n", transaction)
		if processedTransactions.Contains(transaction.AsSha256()) || transaction.Timestamp.Before(startTime) {
			continue
		}
		destinationAddresses, ok := mixerConfig.registeredTransactions.Load(transaction.ToAddress)
		if !ok {
			continue
		}

		transactionStartTime := time.Now()

		// If the transaction is new, has been posted after we started listening, and is to an active deposit address
		// then we will proceed with processing it. First step is to send the amount to the house address.
		_, err := mixerConfig.api.SendCoin(transaction.FromAddress, mixerConfig.houseAddress, transaction.Amount)

		// If sending to the house address fails, another polling pass will attempt to send again.
		if err != nil {
			continue
		}

		// Mix this money and dole it out to destinationAddresses
		mixerConfig.mixToAddresses(destinationAddresses, transaction.Amount, transactionStartTime)

		processedTransactions.Add(transaction.AsSha256())
	}
}

func distributeAmount(destinationAddresses []string, amount string) (float64, []float64, error) {
	// TODO: better random generated. Right now, biggest number first always likely
	var amounts = make([]float64, len(destinationAddresses))
	balance, err := strconv.ParseFloat(amount, 64)

	fee := balance * jobcoin.Fee
	balance -= fee

	for i := range destinationAddresses {
		randomNum := rand.Float64() * balance

		amounts[i] = randomNum
		balance -= randomNum
	}

	fee += balance
	return fee, amounts, err
}

func (mixerConfig *MixerConfig) mixToAddresses(destinationAddresses []string, amount string, startTime time.Time) {
	fmt.Printf("Transaction start time: %v \n", startTime)
	feeCollected, amounts, err := distributeAmount(destinationAddresses, amount)
	mixerConfig.collectedFees += feeCollected

	if err != nil || len(amounts) != len(destinationAddresses) {
		return
	}

	// TODO: should I send multiple transactions to the same address?
	// Or should there be one job per deposit address
	// Randomly split up the amount across the addresses
	for i, address := range destinationAddresses {
		go func(destinationAddress string, amountToSend float64) {
			fmt.Printf("Create job for %s \n", destinationAddress)
			job := func() error {
				sendingAmount := fmt.Sprintf("%f", amountToSend)
				fmt.Printf("Sending %s to %s \n", sendingAmount, destinationAddress)
				_, err := mixerConfig.api.SendCoin(mixerConfig.houseAddress, destinationAddress, sendingAmount)
				return err
			}
			mixerConfig.dispatcher.AddJob(job, time.Now())
		}(address, amounts[i])
	}
}

// sendHandler awaits input from th
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
		// TODO: Perhaps registered transactions should have an expiration on them
		mixerConfig.registeredTransactions.Store(depositAddress, strings.Split(strings.ToLower(addresses), ","))

		go func() {
			mixerConfig.pollAddress(depositAddress)
		}()
	}

	respondWithJSON(w, http.StatusCreated, "Received transaction")
}

func waitForShutdown(srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	// Block until we receive our signal.
	<-interruptChan
	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

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
	w.Write(response)
}