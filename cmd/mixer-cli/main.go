package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/oliviabarnett/mixer/mixerlib"
	"log"
	"net/http"
	"os"
	"strings"
)

func inputDepositAddresses(svar string) string {
	trimmed := strings.TrimSpace(svar)

	// If user enters nothing, remind them of the instructions.
	if trimmed == "" {
		presentInstructions(Welcome)
	}

	return strings.ToLower(trimmed)
}

type Instruction int

const (
	Welcome Instruction = iota
	Send
)

func presentInstructions(phase Instruction) {
	var instruction string
	switch phase {
	case Welcome:
		instruction = `
Welcome to the Jobcoin mixer!
Please enter a comma-separated list of new, unused Jobcoin addresses
where your mixed Jobcoins will be sent. Example:
	./bin/mixer --addresses=bravo,tango,delta
`
	case Send:
		instruction = `You may now send Jobcoins to address DEPOSITADDRESS.
		They will be mixed into ADDRESSES and sent to your destination addresses.`
	}
	fmt.Println(instruction)
}

func main() {
	// Spin up the Mixer service
	go func() {
		mixerlib.ServeMixer()
	}()

	// Handle user input. This _could_ be another service
	input := bufio.NewScanner(os.Stdin)
	presentInstructions(Welcome)
	for input.Scan() {
		addresses := inputDepositAddresses(input.Text())
		depositAddress := uuid.NewString()

		presentInstructions(Send)

		var url = fmt.Sprintf("http://localhost:8080/send")
		var jsonStr = fmt.Sprintf("{\"deposit\":\"%s\", \"addresses\":\"%s\"}", depositAddress, addresses)
		log.Println("json %s", jsonStr)

		req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonStr)))
		req.Header.Set("Content-Type", "application/json")
		if err != nil {
			panic(err)
		}

		client := &http.Client{}
		client.Do(req)
	}
}

// 1. Input transaction: [addresses] and amount
// 2. "Register" transaction and send back deposit address and key
// 3. Scan for new transactions. Check every ~ five min or something? Check for transactions.
//    When we get a transaction that is in our "registered" list, pop it out of registered list and process it
// 4. Process transaction --> call separate transacts

// class able to hit their endpoints

// Two entry boxes for input data
