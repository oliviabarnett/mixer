package clientlib

import (
	"bytes"
	"encoding/json"
	jobcoin "github.com/oliviabarnett/mixer"
	"github.com/oliviabarnett/mixer/internal"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

//TestClient returns *http.Client with Transport replaced to avoid making real calls
func NewMockClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestGetAddressInfo(t *testing.T) {
	transaction := internal.Transaction{
		Timestamp: time.Now(),
		ToAddress: "to",
		FromAddress: "from",
		Amount: "50",
	}

	addressInfo := internal.AddressInfo{
		Balance: "100",
		Transactions: []internal.Transaction{transaction},
	}

	address := "address"

	client := NewMockClient(func(req *http.Request) *http.Response {
		desiredEndpoint := jobcoin.AddressesEndpoint + "/" + address
		if req.URL.String() != desiredEndpoint {
			t.Errorf("GetAddressInfo hits endpoint %s, want: %s.", req.URL.String(), desiredEndpoint)
		}

		jsonValue, _ := json.Marshal(addressInfo)
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBuffer(jsonValue)),
			Header:     make(http.Header),
		}
	})
	receivedAddressInfo, err := getAddressInfo(client, address)
	if err != nil {
		t.Errorf("GetAddressInfo fails with error %s", err)
	}
	if addressInfo.Balance != receivedAddressInfo.Balance {
		t.Errorf("GetAddressInfo sent balance %s, but should have sent: %s.", addressInfo.Balance, receivedAddressInfo.Balance)
	}
	if len(addressInfo.Transactions) != 1 {
		t.Errorf("GetAddressInfo sent %d transactions but should have sent 1", len(addressInfo.Transactions))
	}

	if addressInfo.Transactions[0].AsSha256() != receivedAddressInfo.Transactions[0].AsSha256() {
		t.Errorf("GetAddressInfo sent transaction %v, but should have sent: %v.", addressInfo.Transactions[0], receivedAddressInfo.Transactions[0])
	}
}

func TestSendCoin(t *testing.T) {
	fromAddress := "fromAddress"
	toAddress :=  "toAddress"
	amount := "123"
	client := NewMockClient(func(req *http.Request) *http.Response {
		desiredEndpoint := jobcoin.TransactionEndpoint
		if req.URL.String() != desiredEndpoint {
			t.Errorf("SendCoin hits endpoint %s, want: %s.", req.URL.String(), desiredEndpoint)
		}

		var sentInfo map[string]string
		decoder := json.NewDecoder(req.Body)

		err := decoder.Decode(&sentInfo)
		if err != nil {
			t.Errorf("SendCoin fails to decode sent data with error %s", err)
		}

		if sentInfo["fromAddress"] != fromAddress {
			t.Errorf("SendCoin sent from address %s, but should have sent from %s.", sentInfo["fromAddress"], fromAddress)
		}
		if sentInfo["toAddress"] != toAddress {
			t.Errorf("SendCoin sent tp address %s, but should have sent to %s.", sentInfo["toAddress"], toAddress)
		}
		if sentInfo["amount"] != amount {
			t.Errorf("SendCoin sent %s jobCoin, but should have sent: %s.", sentInfo["amount"], amount)
		}

		return &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
		}
	})
	_, err := sendCoin(client, fromAddress, toAddress, amount)
	if err != nil {
		t.Errorf("SendCoin fails with error %s", err)
	}
}

func TestGetTransactions(t *testing.T) {
	transactions := []internal.Transaction{
		{
			Timestamp:   time.Now(),
			ToAddress:   "to",
			FromAddress: "from",
			Amount:      "53",
		},
		{
			Timestamp:   time.Now(),
			ToAddress:   "to",
			FromAddress: "from",
			Amount:      "5",
		},
	}

	client := NewMockClient(func(req *http.Request) *http.Response {
		desiredEndpoint := jobcoin.TransactionEndpoint
		if req.URL.String() != desiredEndpoint {
			t.Errorf("GetTransactions hits endpoint %s, want: %s.", req.URL.String(), desiredEndpoint)
		}

		jsonValue, _ := json.Marshal(transactions)

		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBuffer(jsonValue)),
			Header:     make(http.Header),
		}
	})
	receivedTransactions, err := getTransactions(client)
	if err != nil {
		t.Errorf("GetTransactions fails with error %s", err)
	}
	if len(receivedTransactions) != 2 {
		t.Errorf("GetTransactions sent %d transactions but should have sent 2", len(receivedTransactions))
	}
	if receivedTransactions[0].AsSha256() != transactions[0].AsSha256() || receivedTransactions[1].AsSha256() != transactions[1].AsSha256() {
		t.Errorf("GetAddressInfo sent transactions %v, but should have sent: %v.", receivedTransactions, transactions)
	}
}