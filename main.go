package main

import (
	"database/sql"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("unable to load .env file")
	}

	db, err := sql.Open("sqlite3", "./upstream.db")
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return
	}

	dbLogger := setupLogger(db)

	config := NewConfig()

	if err = dbInit(db, dbLogger, config); err != nil {
		dbLogger.Fatal().Err(err).Msg("Failed to initialize database")
		return
	}

	proxy := NewProxy(config, db, dbLogger)
	if proxy == nil {
		dbLogger.Fatal().Msg("Failed to create proxy")
		return
	}

	defer func() {
		if err = proxy.Close(); err != nil {
			dbLogger.Error().Err(err).Msg("Failed to close proxy")
		}
	}()

	// Start HTTP server in a goroutine
	go func() {
		if err = proxy.HTTPServer(); err != nil {
			dbLogger.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	// Start proxy server
	if err = proxy.Start(); err != nil {
		dbLogger.Fatal().Err(err).Msg("Failed to start proxy server")
	}
}
