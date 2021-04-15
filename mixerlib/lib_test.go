package mixerlib

import (
	"bytes"
	"fmt"
	"github.com/oliviabarnett/mixer"
	"github.com/oliviabarnett/mixer/clientlib"
	"io/ioutil"
	"net/http"
	"testing"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func TestMixToAddresses(t *testing.T) {
	client := NewTestClient(func(req *http.Request) *http.Response {
		// Test request parameters
		if req.URL.String() != jobcoin.TransactionEndpoint {
			t.Errorf("Mix to addresses hits endpoint %s, want: %s.", req.URL.String(), jobcoin.TransactionEndpoint)
		}
		return &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
			// Must be set to non-nil value or it panics
			Header:     make(http.Header),
		}
	})
	// This won't work right now because API is global
	api := clientlib.API{Client: client}
	mixToAddresses([]string{"alpha", "bravo", "charlie"}, "50")
	fmt.Println("API: ", api)
}


func TestDistributeAmount(t *testing.T) {
	destinationAddresses := []string{"A", "B", "C"}
	amount := fmt.Sprintf("%f", 10.0)
	feeCollected, amounts, err := DistributeAmount(destinationAddresses, amount)

	var sum float64 = 0
	for _, amount := range amounts {
		sum += amount
	}
	sum += feeCollected
	if fmt.Sprintf("%f", sum) != amount || err != nil {
		t.Errorf("Distributing amount over addresses. Sums to %f, want: %s.", sum, amount)
	}
}