package main

import (
	"github.com/google/uuid"
	"sync"
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
