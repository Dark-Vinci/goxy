package main

import (
	"database/sql"
	"strings"

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

	// Create users table
	_, err = db.Exec(createSQLLog)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create db logger table")
	}

	// Create users table
	_, err = db.Exec(createUpstreamHealth)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create db logger table")
	}

	// Insert sample users
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(config.adminPassword), bcrypt.DefaultCost)

	if _, err = db.Exec("INSERT OR REPLACE INTO users (username, password, role) VALUES (?, ?, ?)",
		config.adminUser, hashedPassword, UserRoleAdmin); err != nil {
		logger.Fatal().Err(err).Msg("Failed to insert admin user")
		return err
	}

	for _, slave := range config.servers {
		_, err = db.Exec("INSERT OR REPLACE INTO upstreams (addr, role, healthy, lag) VALUES (?, ?, ?, ?)",
			strings.ReplaceAll(slave, ":", "_"), RoleReplica, false, 0)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to insert replica %s", slave)
		}
	}

	return nil
}
