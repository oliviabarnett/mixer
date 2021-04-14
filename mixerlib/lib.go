package mixerlib

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/oliviabarnett/mixer/clientlib"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Could this be an LRU cache?
// When a transaction is registered, that means we are actively searching for it.
// Deposit address --> list of addresses
var registeredTransactions = make(map[string][]string)
var houseAddress string
var api clientlib.API

func ServeMixer() {
	// On each mixer boot, create a new house address and a new API client for requests to JobCoin
	houseAddress = uuid.NewString()
	api = clientlib.API{http.DefaultClient}

	r := mux.NewRouter()
	r.HandleFunc("/send", sendHandler)
	http.Handle("/", r)

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Start Server and await mixing requests
	go func() {
		log.Println("Starting the Server")
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	waitForShutdown(srv)
}

// Watch the address that the user promised to send JobCoin to for a certain amount time.
// If JobCoin hasn't been sent in that window, abandon the job.
func pollAddress(address string) {
	ticker := time.NewTicker(time.Second * 5)

	defer ticker.Stop()
	done := make(chan bool)
	go func() {
		time.Sleep(600 * time.Second)
		done <- true
	}()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// TODO: what about OLD transactions
			addressInfo, _ := api.GetAddressInfo(address)
			if len(addressInfo.Transactions) > 0 && processTransactions(address, addressInfo.Balance) {
				return
			}
		}
	}
}

// Processes transaction and returns true if we should cease polling for this address
func processTransactions(address string, amount string) bool {
	// Check that we have this address registered (ie awaiting mixing to destinationAddresses)
	destinationAddresses, ok := registeredTransactions[address]
	if !ok { return false }

	// With current implementation, will need to re-hit the mixer with intention to distribute coins to addresses
	// TODO: check if we need to be able to continually send to this address
	delete(registeredTransactions, address)

	// Move amount from deposit address to house address
	api.SendCoin(address, houseAddress, amount)

	// Mix this money and dole it out to destinationAddresses
	mixToAddresses(destinationAddresses, amount)
	return true
}

func mixToAddresses(destinationAddresses []string, amount string) {
	fmt.Println("mixing %s to %v", amount, destinationAddresses)
}

func sendHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("hit send handler: ")

	var receivedData map[string]string
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&receivedData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	defer r.Body.Close()

	addresses, ok := receivedData["addresses"]
	if !ok { return }

	depositAddress, ok := receivedData["deposit"]
	if !ok { return }

	// Save new transactions
	// What if you want to send to A and B but we already have those registered...
	// Hashmap won't work in this case. Another reason to use LRU cache?
	_, ok = registeredTransactions[depositAddress]
	if !ok {
		// TODO: Perhaps registered transactions should have an expiration on them
		registeredTransactions[depositAddress] = strings.Split(strings.ToLower(addresses), ",")

		// Watch this new address for a duration of time for deposits.
		go func() {
			pollAddress(depositAddress)
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

	log.Println("Shutting down")
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

