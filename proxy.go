package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
)

// Proxy represents the PostgreSQL proxy
type Proxy struct {
	config      *Config
	connCounter uint64 // Atomic counter for connection IDs
	mu          sync.Mutex
	next        int
}

// NewProxy creates a new Proxy instance
func NewProxy(config *Config) *Proxy {
	return &Proxy{config: config}
}

// Start runs the proxy server
func (p *Proxy) Start() error {
	listener, err := net.Listen("tcp", p.config.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", p.config.listenAddr, err)
	}
	defer listener.Close()

	log.Printf("Proxy listening on %s, forwarding to %s", p.config.listenAddr, p.config)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		connID := atomic.AddUint64(&p.connCounter, 1)

		go p.handleConnection1(clientConn, connID)
	}
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
