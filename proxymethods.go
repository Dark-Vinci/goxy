package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

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
			ID:        uuid.New(),
			Sql:       nil,
			CreatedAt: time.Now(),
			ctx:       nil,
			connID:    atomic.AddUint64(&p.connCounter, 1),
			requestID: uuid.New(),
			UserID:    uuid.UUID{},
			conn:      clientConn,
		}

		go p.handleConnection(request)
	}
}

func (p *Proxy) Close() error {
	// this stops all goroutines(health check, forwarding)
	p.cancel()

	var errs []error

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
