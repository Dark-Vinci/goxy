package main

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

func dbInit(db *sql.DB, logger zerolog.Logger, config *Config) error {
	// Create a health check table
	_, err := db.Exec(createHealthChecks)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create health check table")
	}

	// Create a users' table
	_, err = db.Exec(createUserTable)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create users table")
	}

	// Create a log entry table
	_, err = db.Exec(createLogEntryTable)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create db logger table")
	}

	// Create a request table
	_, err = db.Exec(createRequestTable)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create request table")
	}

	_, err = db.Exec(createSQLTable)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create SQL table")
	}

	// Insert sample users
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(config.adminPassword), bcrypt.DefaultCost)

	userID := uuid.New().String()
	now := time.Now()

	if _, err = db.Exec(
		`INSERT OR REPLACE INTO users 
	(id, username, password, is_admin, role, created_at, updated_at, deleted_at) 
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID,
		config.adminUser,
		hashedPassword,
		1, // is_admin = true
		UserRoleAdmin,
		now, // created_at
		now, // updated_at
		nil, // deleted_at
	); err != nil {
		logger.Fatal().Err(err).Msg("Failed to insert admin user")
		return err
	}

	return nil
}
