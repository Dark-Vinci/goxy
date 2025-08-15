package main

import (
	"database/sql"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

func dbInit(db *sql.DB, logger zerolog.Logger, config *Config) error {
	// Create upstreams table
	_, err := db.Exec(createUpstreamsSQL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create upstreams table")
	}

	// Create users table
	_, err = db.Exec(createUsersSQL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create users table")
	}

	// Insert sample users
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(config.adminPassword), bcrypt.DefaultCost)

	if _, err = db.Exec("INSERT OR REPLACE INTO users (username, password, role) VALUES (?, ?, ?)",
		config.adminUser, hashedPassword, UserRoleAdmin); err != nil {
		logger.Fatal().Err(err).Msg("Failed to insert admin user")
		return err
	}

	// Insert upstreams
	_, err = db.Exec("INSERT OR REPLACE INTO upstreams (addr, role, healthy, lag) VALUES (?, ?, ?, ?)",
		strings.ReplaceAll(config.master, ":", "_"), RolePrimary, true, 0)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to insert master %s", config.master)
	}

	for _, slave := range config.slaves {
		_, err = db.Exec("INSERT OR REPLACE INTO upstreams (addr, role, healthy, lag) VALUES (?, ?, ?, ?)",
			strings.ReplaceAll(slave, ":", "_"), RoleReplica, false, 0)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to insert replica %s", slave)
		}
	}

	return nil
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	appLogger := logger.With().Str("Thesis", "api").Logger()

	err := godotenv.Load(".env")
	if err != nil {
		logger.Fatal().Msg("Error loading .env file")
		return
	}

	db, err := sql.Open("sqlite3", "./upstream.db")
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to open database: %v", err)
		return
	}

	config := NewConfig()

	if err = dbInit(db, appLogger, config); err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize database")
		return
	}

	proxy := NewProxy(config, db, appLogger)
	if proxy == nil {
		logger.Fatal().Msg("Failed to create proxy")
		return
	}

	defer func() {
		if err := proxy.Close(); err != nil {
			logger.Error().Err(err).Msg("Failed to close proxy")
		}
	}()

	// Start HTTP server in a goroutine
	go func() {
		if err := proxy.HTTPServer(); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	// Start proxy server
	if err = proxy.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start proxy server")
	}
}
