package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Upstream struct {
	Addr    string
	Healthy bool
	Lag     int
	lock    sync.Mutex
	ID      uuid.UUID
	pool    *ConnectionPool
	config  PoolConfig
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func ping(up net.Conn) error {
	_, err := up.Write(encodeSimpleQuery("SELECT 1"))
	if err != nil {
		log.Printf("Health check failed: write error to : %v", err)
		return err
	}

	if err = up.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		log.Printf("Health check failed: set read deadline error to: %v", err)
		return err
	}

	buf := make([]byte, 512)

	if _, err = up.Read(buf); err != nil {
		if !errors.Is(err, io.EOF) {
			log.Printf("Health check failed: read error from: %v", err)
			return err
		}

		fmt.Println("HHERRR")
	}

	return nil
}
