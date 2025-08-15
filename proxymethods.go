package main

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
)

// Start runs the proxy server
func (p *Proxy) Start() error {
	listener, err := net.Listen("tcp", p.config.listenAddr)
	if err != nil {
		p.logger.
			Error().
			Err(err).
			Msg("failed to listen on " + p.config.listenAddr)

		return fmt.Errorf("failed to listen on %s: %v", p.config.listenAddr, err)
	}

	defer func(listener net.Listener) {
		if err := listener.Close(); err != nil {
			p.logger.Error().Err(err).Msgf("Failed to close listener: %v", err)
		}
	}(listener)

	p.logger.Info().Msgf("Proxy listening on %s", p.config.listenAddr)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			p.logger.Error().Err(err).Msgf("Failed to accept connection: %v", err)
			continue
		}

		request := &Request{
			connID:    atomic.AddUint64(&p.connCounter, 1),
			requestId: uuid.New(),
			role:      "",
			userID:    uuid.UUID{},
			conn:      clientConn,
		}

		go p.handleConnection(request)
	}
}

func (p *Proxy) Close() error {
	// this stops all goroutines(health check, forwarding)
	p.cancel()

	// Close master connection
	var errs []error
	if err := p.session.UpPrimary.Conn.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close master %s: %v", p.session.UpPrimary.Addr, err))
	}

	// Close replica connections
	for _, replica := range p.session.Replicas {
		if err := replica.Conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close replica %s: %v", replica.Addr, err))
		}
	}

	// Close database (optional, depending on lifecycle)
	if err := p.sqliteDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close database: %v", err))
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errs)
	}

	return nil
}

// Select backend based on query type
func (p *Proxy) selectBackend(query string) string {
	query = strings.TrimSpace(strings.ToUpper(query))

	if strings.HasPrefix(query, "SELECT") && len(p.config.slaves) > 0 {
		// Round-robin among replicas
		p.mu.Lock()
		addr := p.config.slaves[p.next]
		p.next = (p.next + 1) % len(p.config.slaves)
		p.mu.Unlock()
		return addr
	}

	// Writes always go to master
	return p.config.master
}
