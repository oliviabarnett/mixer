package pkg

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

type Transaction struct {
	Timestamp time.Time
	ToAddress string
	FromAddress string
	Amount string
}

// This is probably NOT performant... not sure how this kind of thing is normally handled in Go
func (transaction Transaction)AsSha256() string {
	h := sha256.New()
	transaction.Timestamp = transaction.Timestamp.Round(0)
	h.Write([]byte(fmt.Sprintf("%v", transaction)))

	return fmt.Sprintf("%x", h.Sum(nil))
}


type AddressInfo struct {
	Balance string
	Transactions []Transaction
}

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
