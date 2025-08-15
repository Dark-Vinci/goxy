package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func main() {
	fmt.Println("Hello, world!")
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

	proxy := NewProxy(config, db, appLogger)
	if err := proxy.Start(); err != nil {
		logger.Fatal().Err(err).Msgf("Failed to start proxy: %v", err)
		return
	}

	if err = proxy.Close(); err != nil {
		logger.Fatal().Err(err).Msgf("Failed to close proxy: %v", err)
		return
	}
}
