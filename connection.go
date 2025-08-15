package main

import (
	"bufio"
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

	// Wrap client connection in a buffered reader
	clientReader := bufio.NewReader(request.conn)

	// Peek the first message to select backend
	peekBytes, err := clientReader.Peek(16384) // or reasonable peek size
	if err != nil && err != io.EOF {
		p.logger.Warn().Err(err).Msgf("Failed to peek client startup message: %v", err)
		return
	}

	query := string(peekBytes)

	// Connect to selected PostgreSQL backend
	// todo; work more on this part
	upstream := handleClientQuery(nil, query)
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
