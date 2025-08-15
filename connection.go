package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"
)

// defer closing client connection
// peek the type of request it is [SELECT,...]
// select an appropriate backend server
// lock to prevent concurrent access to the same backend
// get the locked connection instance
// defer unlocking the connection
// perform other actions with the postgres connection
func (p *Proxy) handleConnection(clientConn net.Conn, connID uint64) {
	defer func(clientConn net.Conn) {
		if err := clientConn.Close(); err != nil {
			log.Printf("Error closing client connection: %v", err)
		}
	}(clientConn)

	// Wrap client connection in a buffered reader
	clientReader := bufio.NewReader(clientConn)

	// Peek the first message to select backend
	peekBytes, err := clientReader.Peek(16384) // or reasonable peek size
	if err != nil && err != io.EOF {
		log.Println("Error peeking client startup message:", err)
		return
	}

	query := string(peekBytes)

	// Connect to selected PostgreSQL backend
	upstream := handleClientQuery(nil, query)
	upstream.lock.Lock()
	defer upstream.lock.Unlock()

	serverConn := upstream.Conn

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> PROXY -> PostgreSQL
	go FromClient(clientConn, serverConn, int64(connID), &wg)

	// PostgreSQL -> PROXY -> Client
	go FromDB(serverConn, clientConn, int64(connID), &wg)

	wg.Wait()
	log.Printf("[Conn %d] Connection closed", connID)
}
