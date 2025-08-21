package main

import (
	"context"
	"database/sql"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"thesis/store"
)

// Proxy represents the PostgreSQL proxy
type Proxy struct {
	writePatterns []*regexp.Regexp
	readPatterns  []*regexp.Regexp
	config        *Config
	connCounter   uint64 // Atomic counter for connection IDs
	lock          sync.Mutex
	next          int
	logger        *zerolog.Logger
	sqliteDB      *sql.DB
	ctx           context.Context
	cancel        context.CancelFunc
	pingInterval  time.Duration
	servers       []*Upstream
	unhealthy     []*Upstream
	serverIndex   uint64
	nthCheck      int

	store struct {
		healthCheckStore store.HealthCheckInterface
		userStore        store.UserInterface
		requestStore     store.RequestInterface
		logsStore        store.LogsInterface
		sqlStore         store.SQLInterface
	}
}

// NewProxy creates a new Proxy instance
func NewProxy(config *Config, db *sql.DB, logger zerolog.Logger) *Proxy {
	servers, unhealthy := make([]*Upstream, 0), make([]*Upstream, 0)

	for _, v := range config.servers {
		_, err := net.Dial("tcp", v)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to connect to replica %v: %v", v, err)

			unhealthy = append(unhealthy, &Upstream{
				Addr:    v,
				Healthy: false,
				Lag:     0,
				lock:    sync.Mutex{},
				ID:      uuid.New(),
			})

			continue
		}

		servers = append(servers, &Upstream{
			Addr:    v,
			Healthy: true,
			Lag:     0,
			lock:    sync.Mutex{},
			ID:      uuid.New(),
		})
	}

	ctx, cancel := context.WithCancel(context.Background())

	gormDB, err := gorm.Open(
		sqlite.New(sqlite.Config{
			Conn: db,
		}), &gorm.Config{})

	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
		panic("something went wrong")
	}

	userStore := store.NewUserStore(&logger, gormDB)
	requestStore := store.NewRequestStore(&logger, gormDB)
	healthCheckStore := store.NewHealthCheckStore(&logger, gormDB)
	logsStore := store.NewLogStore(gormDB, &logger)
	sqlStore := store.NewSQLStore(gormDB, &logger)

	p := &Proxy{
		config:       config,
		logger:       &logger,
		sqliteDB:     db,
		servers:      servers,
		ctx:          ctx,
		cancel:       cancel,
		serverIndex:  uint64(0),
		unhealthy:    unhealthy,
		lock:         sync.Mutex{},
		nthCheck:     0,
		pingInterval: time.Duration(config.pingInterval) * time.Second,

		store: struct {
			healthCheckStore store.HealthCheckInterface
			userStore        store.UserInterface
			requestStore     store.RequestInterface
			logsStore        store.LogsInterface
			sqlStore         store.SQLInterface
		}{
			healthCheckStore: healthCheckStore,
			userStore:        userStore,
			requestStore:     requestStore,
			logsStore:        logsStore,
			sqlStore:         sqlStore,
		},
	}

	// Start pinging for each upstream
	p.healthCheck()
	p.initializePatterns()

	return p
}
