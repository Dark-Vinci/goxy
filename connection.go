package main

import (
	"bufio"
	"fmt"
	"io"
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
func (p *Proxy) handleConnection(request *Request) {
	defer func(clientConn net.Conn) {
		if err := clientConn.Close(); err != nil {
			p.logger.Warn().Err(err).Msgf("Failed to close client connection: %v", err)
		}
	}(request.conn)

	fmt.Println("request got here1")

	// Wrap client connection in a buffered reader
	clientReader := bufio.NewReader(request.conn)
	fmt.Println("request got here2")

	// Peek the first message to select backend
	peekBytes, err := clientReader.Peek(8192) // or reasonable peek size
	if err != nil && err != io.EOF {
		p.logger.Warn().Err(err).Msgf("Failed to peek client startup message: %v", err)
		return
	}

	fmt.Println("request got here3")

	query := string(peekBytes)

	// Connect to selected PostgreSQL backend
	// todo; work more on this part
	upstream := handleClientQuery(p.session, query)
	upstream.lock.Lock()
	defer upstream.lock.Unlock()

	serverConn := upstream.Conn

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> PROXY -> PostgreSQL
	go FromClient(request.conn, serverConn, int(request.connID), &wg)

	// PostgreSQL -> PROXY -> Client
	go FromDB(serverConn, request.conn, int(request.connID), &wg)

	wg.Wait()

	p.logger.Info().Msgf("[Conn %d] Connection closed", request.connID)
}
