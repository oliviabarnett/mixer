package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	jobcoin "github.com/oliviabarnett/mixer"
	"github.com/oliviabarnett/mixer/pkg"
	"io"
	"net/http"
	"os"
	"strings"
)

type API interface {
	GetTransactions() ([]internal.Transaction, error)
	GetAddressInfo(address string) (internal.AddressInfo, error)
	SendCoin(fromAddress string, toAddress string, amount string) (string, error)
}

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

func inputDepositAddresses(svar string) string {
	trimmed := strings.TrimSpace(svar)

	// If user enters nothing, remind them of the instructions.
	if trimmed == "" {
		welcomeInstructions()
	}

	return strings.ToLower(trimmed)
}

type Instruction int

const (
	Welcome Instruction = iota
	Send
)

func welcomeInstructions() {
	instruction := `
Welcome to the Jobcoin mixer!
Please enter a comma-separated list of new, unused Jobcoin addresses
where your mixed Jobcoins will be sent. Example:
	./bin/mixer --addresses=bravo,tango,delta
`
	fmt.Println(instruction)
}

func main() {
	// Handle user input. This _could_ be another service but for now just console input.
	input := bufio.NewScanner(os.Stdin)
	welcomeInstructions()
	for input.Scan() {
		addresses := inputDepositAddresses(input.Text())
		depositAddress := uuid.NewString()

		directions := fmt.Sprintf("You may now send Jobcoins to address %s. \n They will be mixed into %s and sent to your destination addresses. \n", depositAddress, addresses)
		fmt.Println(directions)
		var url = fmt.Sprintf("http://mixermodule:7070/send")
		var jsonStr = fmt.Sprintf("{\"deposit\":\"%s\", \"addresses\":\"%s\"}", depositAddress, addresses)

		req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonStr)))
		req.Header.Set("Content-Type", "application/json")
		if err != nil {
			panic(err)
		}

		client := &http.Client{}
		client.Do(req)
	}
}