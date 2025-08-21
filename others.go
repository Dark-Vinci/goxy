package main

import (
	"github.com/google/uuid"
	"sync"
)

type Upstream struct {
	Addr    string
	Healthy bool
	Lag     int
	lock    sync.Mutex
	ID      uuid.UUID
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
