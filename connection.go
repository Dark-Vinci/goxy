package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"
)

func (p *Proxy) handleConnection1(clientConn net.Conn, connID uint64) {
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

	//todo: most likely here for upstream
	//pgAddr := p.selectBackend(query)

	//log.Println("Forwarding query to:", pgAddr)

	// Connect to selected PostgreSQL backend
	//serverConn, err := net.Dial("tcp", pgAddr)
	serverConn := **(handleClientQuery(nil, query).Pool)

	defer func(serverConn net.Conn) {
		if err := serverConn.Close(); err != nil {
			log.Printf("Error closing PostgreSQL connection: %v", err)
		}
	}(serverConn)

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> PROXY -> PostgreSQL
	go FromClient(clientConn, serverConn, int64(connID), &wg)

	// PostgreSQL -> PROXY -> Client
	go FromDB(serverConn, clientConn, int64(connID), &wg)

	wg.Wait()
	log.Printf("[Conn %d] Connection closed", connID)
}
