package main

import (
	"database/sql"
	"os"

	"github.com/rs/zerolog"
	"thesis/store"
)

func setupLogger(db *sql.DB) zerolog.Logger {
	// Configure console writer
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"}

	// Configure SQLite writer
	sqlWriter := store.NewSqlWriter(db)

	// Combine writers using MultiLevelWriter
	multi := zerolog.MultiLevelWriter(consoleWriter, sqlWriter)

	// Create logger with timestamp and caller
	logger := zerolog.New(multi).With().Timestamp().Caller().Logger()

	return logger
}
