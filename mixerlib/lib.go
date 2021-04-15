package mixerlib

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	jobcoin "github.com/oliviabarnett/mixer"
	"github.com/oliviabarnett/mixer/clientlib"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// https://stackoverflow.com/questions/57646760/communicating-multiple-containers-running-golang

// Could this be an LRU cache?
// When a transaction is registered, that means we are actively searching for it.
// Deposit address --> list of addresses

type RegisteredTransactions struct {
	sync.RWMutex
	internal map[string][]string
}

func NewRegisteredTransactions() *RegisteredTransactions {
	return &RegisteredTransactions{
		internal: make(map[string][]string),
	}
}

func (rm *RegisteredTransactions) Load(key string) (value []string, ok bool) {
	rm.RLock()
	result, ok := rm.internal[key]
	rm.RUnlock()
	return result, ok
}

func (rm *RegisteredTransactions) Delete(key string) {
	rm.Lock()
	delete(rm.internal, key)
	rm.Unlock()
}

func (rm *RegisteredTransactions) Store(key string, value []string) {
	rm.Lock()
	rm.internal[key] = value
	rm.Unlock()
}

// TODO: how to handle these global variables and make them testable
// ENV variables on docker would be nice I think...
var registeredTransactions *RegisteredTransactions
var houseAddress string
var api clientlib.API
var dispatcher *Dispatcher
var collectedFees float64

func ServeMixer() {

	setUpMixerService()

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
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	dispatcher.Start(10)

	waitForShutdown(srv)
	fmt.Println("Fee Collection Total: ", collectedFees)
	dispatcher.Finished()
}

func setUpMixerService() {
	// On each mixer boot, create a new house address and a new API client for requests to JobCoin
	houseAddress = "House address" // uuid.NewString() // TODO: make UUID when done testing
	registeredTransactions = NewRegisteredTransactions()
	dispatcher = NewDispatcher()
	collectedFees = 0
	api = clientlib.API{Client: http.DefaultClient}
	rand.Seed(time.Now().UnixNano())
}

// PollAddress Watch the address that the user promised to send JobCoin to for a certain amount time.
// If JobCoin hasn't been sent in that window, abandon the job.
func PollAddress(address string) {
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
			// TODO: what about OLD transactions? Can I use the time data here?
			addressInfo, _ := api.GetAddressInfo(address)
			// In the case that we identify a new transaction to the deposit address
			// process this transaction
			if len(addressInfo.Transactions) > 0 && ProcessTransactions(address, addressInfo.Balance) {
				return
			}
		}
	}
}

// ProcessTransactions determines whether the given transaction is waiting for mixing. If it is, the transaction
// is removed from the table of registeredTransactions and the entire balance is deposited in the house address.
func ProcessTransactions(address string, amount string) bool {
	// Check that we have this address registered (ie awaiting mixing to destinationAddresses)
	destinationAddresses, ok := registeredTransactions.Load(address)
	if !ok { return false }

	// Move amount from deposit address to house address
	_, err := api.SendCoin(address, houseAddress, amount)
	if err != nil {
		return false
	}

	// With current implementation, will need to re-hit the mixer with intention to distribute coins to addresses
	// TODO: check if we need to be able to continually send to this address
	registeredTransactions.Delete(address)

	// Mix this money and dole it out to destinationAddresses
	mixToAddresses(destinationAddresses, amount)
	return true
}

func DistributeAmount(destinationAddresses []string, amount string) (float64, []float64, error) {
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

func mixToAddresses(destinationAddresses []string, amount string) {
	//fmt.Println("Mixing %s to %v \n", amount, destinationAddresses)

	feeCollected, amounts, err := DistributeAmount(destinationAddresses, amount)
	collectedFees += feeCollected

	//fmt.Println("Amounts in transactions: %s \n", amounts)
	if err != nil || len(amounts) != len(destinationAddresses) {
		return
	}

	// TODO: should I send multiple transactions to the same address?
	// Or should there be one job per deposit address
	// Randomly split up the amount across the addresses
	for i, address := range destinationAddresses {
		job := func() error {
			fmt.Println("about to access api")
			_, err := api.SendCoin(houseAddress, address, fmt.Sprintf("%f", amounts[i]))
			return err
		}

		dispatcher.AddJob(job)
	}
}

func sendHandler(w http.ResponseWriter, r *http.Request) {
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
	_, ok = registeredTransactions.Load(depositAddress)
	if !ok {
		// TODO: Perhaps registered transactions should have an expiration on them
		registeredTransactions.Store(depositAddress, strings.Split(strings.ToLower(addresses), ","))

		// Watch this new address for a duration of time for deposits.
		go func() {
			PollAddress(depositAddress)
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

	log.Println("Shutting down mixer...")
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

/// SCHEDULER

type Job struct {
	ID int
	F func() error
	time time.Duration
}

type DispatchStatus struct {
	Type string
	ID int
	Status string
}

type Dispatcher struct {
	jobCounter int
	jobQueue chan *Job
	dispatchStatus chan *DispatchStatus
	workQueue chan *Job
	workerQueue chan *Worker
}

type Worker struct {
	ID int
	jobs chan *Job
	dispatchStatus chan *DispatchStatus
	Quit chan bool
}

type JobExecutable func() error

func CreateNewWorker(id int, workerQueue chan *Worker, jobQueue chan *Job, dStatus chan *DispatchStatus) *Worker {
	w := &Worker{
		ID: id,
		jobs: jobQueue,
		dispatchStatus: dStatus,
	}

	go func() { workerQueue <- w }()
	return w
}

func (w *Worker) Start() {
	go func() {
		for {
			select {
			case job := <- w.jobs:
				fmt.Printf("Worker[%d] executing job[%d].\n", w.ID, job.ID)
				job.F()
				w.dispatchStatus <- &DispatchStatus{Type: "worker", ID: w.ID, Status: "quit"}
				w.Quit <- true
			case <- w.Quit:
				return
			}
		}
	}()
}

func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		jobCounter: 0,
		jobQueue: make(chan *Job),
		dispatchStatus: make(chan *DispatchStatus),
		workQueue: make(chan *Job),
		workerQueue: make(chan *Worker),
	}
	return d
}

// Start executes job after a certain amount of time
// Should the job only be dispatched after a certain amount of time? Or immediately & then sleep for duration?
func (d *Dispatcher) Start(numWorkers int) {
	for i := 0; i<numWorkers; i++ {
		worker := CreateNewWorker(i, d.workerQueue, d.workQueue, d.dispatchStatus)
		worker.Start()
	}

	go func() {
		for {
			select {
			case job := <- d.jobQueue:
				fmt.Printf("Got a job in the queue to dispatch: %d\n", job.ID)
				d.workQueue <- job
			case ds := <- d.dispatchStatus:
				fmt.Printf("Got a dispatch status:\n\tType[%s] - ID[%d] - Status[%s]\n", ds.Type, ds.ID, ds.Status)
				if ds.Type == "worker" {
					if ds.Status == "quit" {
						d.jobCounter--
					}
				}
			}
		}
	}()
}

func (d *Dispatcher) AddJob(je JobExecutable) {
	j := &Job{ID: d.jobCounter, F: je}
	go func() { d.jobQueue <- j }()
	d.jobCounter++
	fmt.Printf("jobCounter is now: %d\n", d.jobCounter)
}

func (d *Dispatcher) Finished() bool {
	fmt.Printf("Finished Dispatcher \n")
	return d.jobCounter < 1
}