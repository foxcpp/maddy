package resource

import (
	"sync"
)

// Singleton represents a set of resources identified by an unique key.
type Singleton[T Resource] struct {
	lock      sync.RWMutex
	resources map[string]T
}

func NewSingleton[T Resource]() *Singleton[T] {
	return &Singleton[T]{
		resources: make(map[string]T),
	}
}

func (s *Singleton[T]) GetOpen(key string, open func() (T, error)) (T, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	existing, ok := s.resources[key]
	if ok {
		return existing, nil
	}

	res, err := open()
	if err != nil {
		var empty T
		return empty, err
	}

	s.resources[key] = res

	return res, nil
}

func (s *Singleton[T]) CloseUnused(isUsed func(key string) bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for key, res := range s.resources {
		if isUsed(key) {
			continue
		}
		res.Close()
		delete(s.resources, key)
	}

	return nil
}

func (s *Singleton[T]) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for key, res := range s.resources {
		res.Close()
		delete(s.resources, key)
	}

	return nil
}
