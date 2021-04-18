package jobcoin

// Configuration defines a base minimum configuration for the jobcoin mixer
const (
	BaseURL             = "https://jobcoin.gemini.com/snugly-harmless/api"
	AddressesEndpoint   = BaseURL + "/addresses"
	TransactionEndpoint = BaseURL + "/transactions"
	DepositInterval 	= 30.0 // The max duration within which the transactions should be made once scheduled in seconds
	Fee					= 0.01 // The fee to be added as a percent
	TransactionWindow 	= 600 // Maximum amount of time that the Mixer should scan a certain deposit address
)