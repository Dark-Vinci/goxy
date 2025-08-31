package main

import (
	"net"
	"sync"
)

// defer closing client connection
// peek the type of request it is [SELECT, ...]
// select an appropriate backend server
// lock to prevent concurrent access to the same backend
// get the locked connection instance
// defers unlocking the connection
// perform other actions with the postgres connection
func (p *Proxy) handleConnection(request *Request) {
	// defer closing client connection
	defer func(clientConn net.Conn) {
		if err := clientConn.Close(); err != nil {
			p.logger.Warn().Err(err).Msgf("Failed to close client connection: %v", err)
		}
	}(request.conn)

	// defer insertion into Request, generated SQLS
	defer func() {
		go func() {
			if err := p.InsertRequest(*request); err != nil {
				p.logger.Error().Err(err).Msgf("Failed to insert request into database: %v", err)
			}

			if err := p.InsertSQLS(*request); err != nil {
				p.logger.Error().Err(err).Msgf("Failed to insert sqls into database: %v", err)
			}
		}()
	}()

	// read a startup message
	rawMessage, _ := readStartupMessage(request.conn)

	//parse the startup message
	params, protocol := parseTheStartupMessage(rawMessage)

	if _, ok := params[TokenKey]; !ok {
		_ = writeError(request.conn, "28000", "FATAL", "token is missing")
		return
	}

	token, _ := params[TokenKey]

	// validate the token
	_, role, err := p.validateJWT(request.ctx, request.requestID, token)
	if err != nil {
		_ = writeError(request.conn, "28000", "FATAL", "token is invalid")
		return
	}

	// delete/modify token from params
	delete(params, "token")

	//build a startup message
	newMessage := buildStartupMessage(params, protocol)

	if len(p.servers) == 0 {
		_ = writeError(request.conn, "08006", "FATAL", "all servers are down, please try again later")
		return
	}

	// Connect to the selected PostgresSQL backend
	upstream := p.getNextServer()
	if upstream == nil {
		_ = writeError(request.conn, "08004", "FATAL", "no available upstream servers")
		return
	}

	// get a connection from the pool
	conn, err := upstream.pool.Get(request.ctx)
	if err != nil {
		_ = writeError(request.conn, "08001", "ERROR", "cannot get backend connection")
		return
	}

	// defer releasing the connection to the pool
	defer upstream.pool.Release(conn)

	// set the server address in the request
	request.serverAddr = &upstream.Addr

	// Send a startup message to PostgresSQL
	_, err = conn.Write(newMessage)
	if err != nil {
		_ = writeError(request.conn, "", "", "something went wrong2")
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> PROXY
	go p.frontend(conn, request, int(request.connID), role, &wg)

	// PROXY -> Client
	go p.backend(conn, request.conn, int(request.connID), &wg)

	// wait for both goroutines to finish
	wg.Wait()

	p.logger.Info().Msgf("[Conn %d] Connection closed", request.connID)
}
