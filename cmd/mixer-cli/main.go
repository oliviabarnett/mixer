package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/oliviabarnett/mixer/mixerlib"
	"net/http"
	"os"
	"strings"
)

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
	// Spin up the Mixer service
	go func() {
		mixerlib.ServeMixer()
	}()

	// Handle user input. This _could_ be another service
	input := bufio.NewScanner(os.Stdin)
	welcomeInstructions()
	for input.Scan() {
		addresses := inputDepositAddresses(input.Text())
		depositAddress := uuid.NewString()

		directions := fmt.Sprintf("You may now send Jobcoins to address %s. \n They will be mixed into %s and sent to your destination addresses. \n", depositAddress, addresses)
		fmt.Println(directions)

		var url = fmt.Sprintf("http://localhost:8080/send")
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
