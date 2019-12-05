package config

import (
	"strconv"
	"time"

	"github.com/foxcpp/maddy/internal/limiters"
)

// GlobalRateLimit reads '... <burst> <interval>' config directive and returns
// limiters.Rate created using arguments.
func GlobalRateLimit(m *Map, node *Node) (interface{}, error) {
	if len(node.Args) != 2 {
		return nil, m.MatchErr("need two arguments: <burst> <interval>")
	}

	burst, err := strconv.Atoi(node.Args[0])
	if err != nil {
		return nil, m.MatchErr("%v", err)
	}

	interval, err := time.ParseDuration(node.Args[1])
	if err != nil {
		return nil, m.MatchErr("%v", err)
	}

	return limiters.NewRate(burst, interval), nil
}

func NoGlobalRateLimit() (interface{}, error) {
	return limiters.NewRate(0, 0), nil
}

// KeyRateLimit reads '... <burst> <interval>' config directive and returns
// limiters.RateSet created using arguments, maxBuckets is currently hardcoded
// to be 20010 (slightly higher than the default max_recipients value).
func KeyRateLimit(m *Map, node *Node) (interface{}, error) {
	if len(node.Args) != 2 {
		return nil, m.MatchErr("need two arguments: <burst> <interval>")
	}

	burst, err := strconv.Atoi(node.Args[0])
	if err != nil {
		return nil, m.MatchErr("%v", err)
	}

	interval, err := time.ParseDuration(node.Args[1])
	if err != nil {
		return nil, m.MatchErr("%v", err)
	}

	return limiters.NewRateSet(burst, interval, 20010), nil
}

func NoKeyRateLimit() (interface{}, error) {
	return limiters.NewRateSet(0, 0, 20010), nil
}

// ConcurrencyLimit reads '... <max>' config directive and returns limiters.Semaphore
// created using arguments.
func ConcurrencyLimit(m *Map, node *Node) (interface{}, error) {
	if len(node.Args) != 0 {
		return nil, m.MatchErr("need two arguments: <max>")
	}

	max, err := strconv.Atoi(node.Args[0])
	if err != nil {
		return nil, m.MatchErr("%v", err)
	}

	return limiters.NewSemaphore(max), nil
}

func NoConcurrencyLimit() (interface{}, error) {
	return limiters.NewSemaphore(0), nil
}
