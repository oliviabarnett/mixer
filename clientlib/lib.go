package clientlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/oliviabarnett/mixer"
	"github.com/oliviabarnett/mixer/internal"
	"io"
	"net/http"
)

// API exposes the different functions used to interact with the JobCoin API
type API interface {
	GetTransactions() ([]internal.Transaction, error)
	GetAddressInfo(address string) (internal.AddressInfo, error)
	SendCoin(fromAddress string, toAddress string, amount string) (string, error)
}

// JobCoinAPI specifies the method of contact with the API
type JobCoinAPI struct {
	Client  *http.Client
}

// GetTransactions gets the list of all JobCoin transactions
func (api JobCoinAPI)GetTransactions() ([]internal.Transaction, error) {
	return getTransactions(api.Client)
}

func getTransactions(client *http.Client) ([]internal.Transaction, error) {
	response, err := client.Get(jobcoin.TransactionEndpoint)
	if err != nil {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Failed to successfully close reader in getTransactions")
		}
	}(response.Body)
	var transactions []internal.Transaction
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&transactions)
	return transactions, err
}

// GetAddressInfo gets the balance and list of transactions for an address.
func (api JobCoinAPI)GetAddressInfo(address string) (internal.AddressInfo, error) {
	return getAddressInfo(api.Client, address)
}

func getAddressInfo(client *http.Client, address string) (internal.AddressInfo, error) {
	response, err := client.Get(jobcoin.AddressesEndpoint + "/" + address)
	if err != nil {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Failed to successfully close reader in getAddressInfo")
		}
	}(response.Body)
	var addressInfo internal.AddressInfo
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&addressInfo)
	return addressInfo, err
}

// SendCoin posts a specified amount of JobCoin from one address to another
func (api JobCoinAPI)SendCoin(fromAddress string, toAddress string, amount string) (string, error) {
	return sendCoin(api.Client, fromAddress, toAddress, amount)
}

func sendCoin(client *http.Client, fromAddress string, toAddress string, amount string) (string, error) {
	values := map[string]string{"fromAddress": fromAddress, "toAddress": toAddress, "amount": amount}
	jsonValue, _ := json.Marshal(values)
	response, err := client.Post(jobcoin.TransactionEndpoint, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		panic(err)
	}
	return response.Status, err
}