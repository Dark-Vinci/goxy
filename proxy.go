package main

import (
	"context"
	"database/sql"
	"net"
	"regexp"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
)

// Proxy represents the PostgreSQL proxy
type Proxy struct {
	writePatterns []*regexp.Regexp
	readPatterns  []*regexp.Regexp
	config        *Config
	connCounter   uint64 // Atomic counter for connection IDs
	mu            sync.Mutex
	next          int
	session       *Session
	logger        *zerolog.Logger
	sqliteDB      *sql.DB
	ctx           context.Context
	cancel        context.CancelFunc
	pingInterval  time.Duration
}

// NewProxy creates a new Proxy instance
func NewProxy(config *Config, db *sql.DB, logger zerolog.Logger) *Proxy {
	session := &Session{}

	master, err := net.Dial("tcp", config.master)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to connect to master: %v", err)
		return nil
	}

	session.UpPrimary = &Upstream{
		Addr:    config.master,
		Role:    RolePrimary,
		Healthy: true,
		Lag:     0,
		Conn:    master,
		lock:    sync.Mutex{},
	}

	for _, v := range config.slaves {
		replica, err := net.Dial("tcp", v)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to connect to replica %v: %v", v, err)
			continue
		}

		session.Replicas = append(session.Replicas, &Upstream{
			Addr:    v,
			Role:    RoleReplica,
			Healthy: false,
			Lag:     0,
			Conn:    replica,
			lock:    sync.Mutex{},
		})
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Proxy{
		config:       config,
		logger:       &logger,
		sqliteDB:     db,
		session:      session,
		ctx:          ctx,
		cancel:       cancel,
		pingInterval: time.Duration(config.pingInterval) * time.Minute,
	}

	// Start pinging for each upstream
	p.healthCheck()

	return p
}
