package main

import (
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

	// read startup message
	rawMessage, _ := readStartupMessage(request.conn)

	//parse the startup message
	params, protocol := parseTheStartupMessage(rawMessage)

	if _, ok := params["token"]; !ok {
		_ = writeError(request.conn, "42883", "invalid_authorization_specification", "token is missing")
		return
	}

	token, _ := params["token"]

	_, role, err := p.validateJWT(token)
	if err != nil {
		_ = writeError(request.conn, "42883", "invalid_authorization_specification", "token is invalid")
		return
	}

	// delete/modify token from params
	delete(params, "token")

	//build startup message
	newMessage := buildStartupMessage(params, protocol)

	// Connect to selected PostgreSQL backend
	upstream := p.session.UpPrimary
	upstream.lock.Lock()
	defer upstream.lock.Unlock()

	// Send startup message to PostgreSQL
	_, err = upstream.Conn.Write(newMessage)
	if err != nil {
		_ = writeError(request.conn, "", "", "something went wrong")
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> PROXY -> PostgreSQL
	go FromClient(request.conn, upstream.Conn, int(request.connID), role, &wg)

	// PostgreSQL -> PROXY -> Client
	go FromDB(upstream.Conn, request.conn, int(request.connID), &wg)

	wg.Wait()

	p.logger.Info().Msgf("[Conn %d] Connection closed", request.connID)
}
