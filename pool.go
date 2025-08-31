package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

type PoolConfig struct {
	MaxConnections int           // Maximum number of connections in the pool
	ConnString     string        // PostgreSQL connection string
	MaxIdleTime    time.Duration // Max time a connection can remain idle
	MaxLifetime    time.Duration // Max lifetime of a connection
}

type ConnectionPool struct {
	config      PoolConfig
	connections chan net.Conn
	mutex       sync.Mutex
	closed      bool
}

func NewConnectionPool(config PoolConfig) (*ConnectionPool, error) {
	pool := &ConnectionPool{
		config:      config,
		connections: make(chan net.Conn, config.MaxConnections),
		closed:      false,
	}

	// Initialize the pool with connections
	for i := 0; i < config.MaxConnections; i++ {
		conn, err := net.Dial("tcp", config.ConnString)
		if err != nil {
			// Close any already opened connections
			pool.Close()
			return nil, fmt.Errorf("failed to create connection: %w", err)
		}

		pool.connections <- conn
	}

	return pool, nil
}

func (p *ConnectionPool) Close() {
	p.mutex.Lock()
	if p.closed {
		p.mutex.Unlock()
		return
	}

	p.closed = true
	p.mutex.Unlock()

	close(p.connections)

	for conn := range p.connections {
		_ = conn.Close()
	}
}

// Release returns a connection to the pool.
func (p *ConnectionPool) Release(conn net.Conn) {
	p.mutex.Lock()

	if p.closed {
		p.mutex.Unlock()
		_ = conn.Close()
		return
	}

	p.mutex.Unlock()

	select {
	case p.connections <- conn:
		// Connection returned to pool
	default:
		// Pool is full, close the connection
		_ = conn.Close()
	}
}

// Get retrieves a connection from the pool.
func (p *ConnectionPool) Get(ctx context.Context) (net.Conn, error) {
	p.mutex.Lock()
	if p.closed {
		p.mutex.Unlock()
		return nil, fmt.Errorf("connection pool is closed")
	}

	p.mutex.Unlock()

	select {
	// ping or reconnect to the database
	case conn := <-p.connections:
		// Check if the connection is still valid
		if err := ping(conn); err != nil {
			// Close an invalid connection and create a new one
			_ = conn.Close()

			// reconnect to the database
			newConn, err := net.Dial("tcp", p.config.ConnString)
			if err != nil {
				return nil, fmt.Errorf("failed to replace invalid connection: %w", err)
			}

			return newConn, nil
		}

		return conn, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
