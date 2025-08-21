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

	// defer insertion into Request
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

	// read startup message
	rawMessage, _ := readStartupMessage(request.conn)

	//parse the startup message
	params, protocol := parseTheStartupMessage(rawMessage)

	if _, ok := params[TokenKey]; !ok {
		_ = writeError(request.conn, "42883", "invalid_authorization_specification", "token is missing")
		return
	}

	token, _ := params[TokenKey]

	_, role, err := p.validateJWT(request.ctx, request.requestID, token)
	if err != nil {
		_ = writeError(request.conn, "42883", "invalid_authorization_specification", "token is invalid")
		return
	}

	// delete/modify token from params
	delete(params, "token")

	//build startup message
	newMessage := buildStartupMessage(params, protocol)

	if len(p.servers) == 0 {
		_ = writeError(request.conn, "42883", "invalid_authorization_specification", "all servers are down, please try again later")
		return
	}

	// Connect to selected PostgreSQL backend
	upstream := p.getNextServer()
	if upstream == nil {
		_ = writeError(request.conn, "", "", "something went wrong1")
		return
	}

	upstream.lock.Lock()
	defer upstream.lock.Unlock()

	conn, err := net.Dial("tcp", upstream.Addr)
	if err != nil {
		_ = writeError(request.conn, "", "", "something went wrong2")
		return
	}

	// set the server address in the request
	request.serverAddr = &upstream.Addr

	// Send startup message to PostgreSQL
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

	wg.Wait()

	p.logger.Info().Msgf("[Conn %d] Connection closed", request.connID)
}
