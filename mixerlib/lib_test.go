package main

import (
	"bytes"
	"fmt"
	"github.com/oliviabarnett/mixer"
	"github.com/oliviabarnett/mixer/internal"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewMockClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func NewMockDispatcher(jobQueue chan *internal.Job, dispatchStatus chan *internal.DispatchStatus, workQueue chan *internal.Job, workerQueue chan *internal.Worker) *internal.Dispatcher {
	d := &internal.Dispatcher{
		JobCounter:     0,
		JobQueue:       jobQueue,
		DispatchStatus: dispatchStatus,
		WorkQueue:      workQueue,
		WorkerQueue:    workerQueue,
	}
	return d
}

type MockAPI struct {
	Client  *http.Client
	addressesToFill *[]string
}

func (api MockAPI)GetTransactions() ([]internal.Transaction, error) {
	log.Println("GETTRANSACTIONS")
	transactions := []internal.Transaction{}
	return transactions, nil
}

/// Get the balance and list of transactions for an address.
func (api MockAPI)GetAddressInfo(address string) (internal.AddressInfo, error) {
	log.Println("GETADDRESSINFO: address: ", address)
	addressInfo := internal.AddressInfo{}
	return addressInfo, nil
}

/// Send a specified amount of JobCoin from one address to another
func (api MockAPI)SendCoin(fromAddress string, toAddress string, amount string) (string, error) {
	log.Println("SENDCOIN: fromAddress: ", fromAddress)
	log.Println(" toAddress: ", toAddress)
	log.Println("amount: ", amount)
	*api.addressesToFill = append(*api.addressesToFill, amount)
	return "200", nil
}

func NewMockMixerConfig(client *http.Client,
						registeredTransactions *internal.RegisteredTransactions,
						jobQueue chan *internal.Job,
						dispatchStatus chan *internal.DispatchStatus,
						workQueue chan *internal.Job,
						workerQueue chan *internal.Worker,
						addressesToFill *[]string) *MixerConfig {
	return &MixerConfig{
		houseAddress:           "House address",
		registeredTransactions: registeredTransactions,
		dispatcher:             NewMockDispatcher(jobQueue, dispatchStatus, workQueue, workerQueue),
		collectedFees:          0,
		api:                    &MockAPI{client, addressesToFill},
	}
}

func TestMixToAddresses(t *testing.T) {
	client := NewMockClient(func(req *http.Request) *http.Response {
		if req.URL.String() != jobcoin.TransactionEndpoint {
			t.Errorf("MixToAddresses hits endpoint %s, want: %s.", req.URL.String(), jobcoin.TransactionEndpoint)
		}

		fmt.Println("HIT CLIENT")
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
			Header:     make(http.Header),
		}
	})

	addressesToFill := make([]string, 0, 5)

	var jobQueue = make(chan *internal.Job)
	var dispatchStatus = make(chan *internal.DispatchStatus)
	var workQueue = make(chan *internal.Job)
	var workerQueue = make(chan *internal.Worker)
	registeredTransactions := internal.NewRegisteredTransactions()

	mixerConfig := NewMockMixerConfig(client, registeredTransactions, jobQueue, dispatchStatus, workQueue, workerQueue, &addressesToFill)
	mixerConfig.mixToAddresses([]string{"alpha", "bravo", "charlie"}, "50", time.Now())

	if mixerConfig.collectedFees == 0 {
		t.Errorf("MixToAddresses not collecting a fee")
	}

	var scheduledJobs []*internal.Job
	for i := range jobQueue {
		scheduledJobs = append(scheduledJobs, i)

		if len(scheduledJobs) == 3 {
			break
		}
	}

	if len(scheduledJobs) != 3 {
		t.Errorf("MixToAddresss fails to schedule 3 jobs, instead schedules %d", len(scheduledJobs))
	}

	mixerConfig.dispatcher.Start(3)

	if len(addressesToFill) == 0 {
		t.Errorf("MixToAddresses fails to distribute %v, ", addressesToFill)
	}
}

func TestDistributeAmount(t *testing.T) {
	destinationAddresses := []string{"A", "B"}
	amount := fmt.Sprintf("%f", 10.0)
	startTime := time.Now()
	maxTime := time.Unix(startTime.Unix() + jobcoin.DepositInterval, 0)
	feeCollected, amounts, times, err := distribute(destinationAddresses, amount, startTime.Unix())

	var sum float64 = 0
	for _, amount := range amounts {
		sum += amount
	}
	sum += feeCollected
	if fmt.Sprintf("%f", sum) != amount || err != nil {
		t.Errorf("Distributing amount over addresses. Sums to %f, want: %s.", sum, amount)
	}

	for _, scheduledTime := range times {
		if scheduledTime.After(maxTime) {
			t.Errorf("Scheduled to send after max allotted time. Scheduled at %s, want before: %s.", scheduledTime.String(), maxTime.String())
		}
		if scheduledTime.Before(startTime) {
			t.Errorf("Scheduled to send after before start time. Scheduled at %s, want after: %s.", scheduledTime.String(), startTime.String())
		}
	}

	fmt.Printf("amounts %v \n", amounts)
}

func TestProcessTransactions(t *testing.T) {
	client := NewMockClient(func(req *http.Request) *http.Response {
		if req.URL.String() != jobcoin.TransactionEndpoint {
			t.Errorf("MixToAddresses hits endpoint %s, want: %s.", req.URL.String(), jobcoin.TransactionEndpoint)
		}

		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
			Header:     make(http.Header),
		}
	})

	targetTransaction := internal.Transaction {
		Timestamp:   time.Now(),
		ToAddress:   "depositAddress",
		FromAddress: "from",
		Amount:      "53",
	}
	notTargetDepositAddressTransaction := internal.Transaction {
		Timestamp:   time.Now(),
		ToAddress:   "someOtherAddress",
		FromAddress: "from",
		Amount:      "53",
	}
	tooOldTransaction := internal.Transaction {
		Timestamp:   time.Date(1996, time.Month(2), 21, 1, 10, 30, 0, time.UTC),
		ToAddress:   "depositAddress",
		FromAddress: "from",
		Amount:      "53",
	}

	transactions := []internal.Transaction{targetTransaction, notTargetDepositAddressTransaction, tooOldTransaction}
	addressesToFill := make([]string, 0, 5)

	var jobQueue = make(chan *internal.Job)
	var dispatchStatus = make(chan *internal.DispatchStatus)
	var workQueue = make(chan *internal.Job)
	var workerQueue = make(chan *internal.Worker)
	registeredTransactions := internal.NewRegisteredTransactions()

	registeredTransactions.Store("depositAddress", []string{"a", "b"})

	mixerConfig := NewMockMixerConfig(client, registeredTransactions, jobQueue, dispatchStatus, workQueue, workerQueue, &addressesToFill)

	processedTransactions := internal.NewSet()
	startTime := time.Date(2000, time.Month(2), 21, 1, 10, 30, 0, time.UTC)
	mixerConfig.processTransactions(transactions, processedTransactions, startTime)

	if !processedTransactions.Contains(targetTransaction.AsSha256()) {
		t.Errorf("Did not identify a valid transaction to desired deposit address. Should have found:  %v", targetTransaction)
	}
	if processedTransactions.Contains(notTargetDepositAddressTransaction.AsSha256()) {
		t.Errorf("Processed a transaction that was not to a desired deposit address. Should not have processed:  %v", notTargetDepositAddressTransaction)
	}
	if processedTransactions.Contains(tooOldTransaction.AsSha256()) {
		t.Errorf("Processed a transaction that was sent before we started scanning. Should not have processed:  %v", tooOldTransaction)
	}
}
