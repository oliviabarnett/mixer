package pkg

import "sync"

var exists = struct{}{}

type Set struct {
	internal map[string]struct{}
	sync.RWMutex
}

func NewSet() *Set {
	s := &Set{}
	s.internal = make(map[string]struct{})
	return s
}

func (s *Set) Add(value string) {
	s.Lock()
	s.internal[value] = exists
	s.Unlock()
}

func (s *Set) Remove(value string) {
	s.Lock()
	delete(s.internal, value)
	s.Unlock()
}

func (s *Set) Contains(value string) bool {
	s.RLock()
	_, c := s.internal[value]
	s.RUnlock()
	return c
}