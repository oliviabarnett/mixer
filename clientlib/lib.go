package clientlib

import (
	"bytes"
	"encoding/json"
	"github.com/oliviabarnett/mixer"
	"net/http"
)

type Transaction struct {
	Timestamp string
	ToAddress string
	FromAddress string
	Amount string
}

type AddressInfo struct {
	Balance string
	Transactions []Transaction
}

type API struct {
	Client  *http.Client
}

/// Get the list of all JobCoin transactions
func (api*API)GetTransactions() ([]Transaction, error) {
	response, err := api.Client.Get(jobcoin.TransactionEndpoint)
	if err != nil {
		panic(err)
	}

	defer response.Body.Close()

	var transactions []Transaction
	decoder := json.NewDecoder(response.Body)

	err = decoder.Decode(&transactions)

	return transactions, err
}

/// Get the balance and list of transactions for an address.
func (api*API)GetAddressInfo(address string) (AddressInfo, error) {
	response, err := api.Client.Get(jobcoin.AddressesEndpoint + "/" + address)

	if err != nil {
		panic(err)
	}

	defer response.Body.Close()

	var addressInfo AddressInfo
	decoder := json.NewDecoder(response.Body)

	err = decoder.Decode(&addressInfo)
	return addressInfo, err
}

/// Send a specified amount of JobCoin from one address to another
func (api*API)SendCoin(fromAddress string, toAddress string, amount string) (string, error) {
	values := map[string]string{"fromAddress": fromAddress, "toAddress": toAddress, "amount": amount}
	jsonValue, _ := json.Marshal(values)
	response, err := api.Client.Post(jobcoin.TransactionEndpoint, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		panic(err)
	}

	return response.Status, err
}