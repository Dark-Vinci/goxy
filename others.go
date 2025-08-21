package main

import (
	"sync"

	"github.com/google/uuid"
)

type Upstream struct {
	Addr    string
	Healthy bool
	Lag     int
	//Conn    net.Conn // your pool wrapper over net.Conn
	lock sync.Mutex
	ID   uuid.UUID
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
