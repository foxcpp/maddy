package resource

import (
	"sync"

	"github.com/foxcpp/maddy/framework/log"
)

// Singleton represents a set of resources identified by an unique key.
type Singleton[T Resource] struct {
	log       *log.Logger
	lock      sync.RWMutex
	resources map[string]T
}

func NewSingleton[T Resource](log *log.Logger) *Singleton[T] {
	return &Singleton[T]{
		log:       log,
		resources: make(map[string]T),
	}
}

func (s *Singleton[T]) GetOpen(key string, open func() (T, error)) (T, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	existing, ok := s.resources[key]
	if ok {
		s.log.DebugMsg("resource reused", "key", key)
		return existing, nil
	}

	res, err := open()
	if err != nil {
		var empty T
		return empty, err
	}

	s.log.DebugMsg("new resource", "key", key)
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
		s.log.DebugMsg("resource released", "key", key)
		res.Close()
		delete(s.resources, key)
	}

	return nil
}

func (s *Singleton[T]) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for key, res := range s.resources {
		s.log.DebugMsg("resource released", "key", key)
		res.Close()
		delete(s.resources, key)
	}

	return nil
}
