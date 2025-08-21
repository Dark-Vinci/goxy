package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

type SqlWriter struct {
	db *sql.DB
}

var _ io.Writer = (*SqlWriter)(nil)

func NewSqlWriter(db *sql.DB) *SqlWriter {
	return &SqlWriter{db: db}
}

func (l *SqlWriter) Write(p []byte) (n int, err error) {
	var evt map[string]interface{}
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	err = d.Decode(&evt)
	if err != nil {
		return 0, fmt.Errorf("cannot decode event: %s", err)
	}

	// Extract fields
	level, _ := evt[zerolog.LevelFieldName].(string)
	timestamp, _ := evt[zerolog.TimestampFieldName].(json.Number)
	message, _ := evt[zerolog.MessageFieldName].(string)
	caller, _ := evt[zerolog.CallerFieldName].(string)

	// Remove standard fields to store remaining as JSON
	delete(evt, zerolog.LevelFieldName)
	delete(evt, zerolog.TimestampFieldName)
	delete(evt, zerolog.MessageFieldName)
	delete(evt, zerolog.CallerFieldName)

	var extraFields []byte
	if len(evt) > 0 {
		extraFields, err = json.Marshal(evt)
		if err != nil {
			fmt.Println("Error marshaling extra fields:", err)
		}
	}

	// Format caller to relative path
	var formattedCaller *string
	if caller != "" {
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, caller); err == nil {
				formattedCaller = &rel
			}
		}
	}

	// Insert log into SQLite
	query := `INSERT INTO log_entries (level, timestamp, caller, message, fields) VALUES (?, ?, ?, ?, ?)`
	_, err = l.db.Exec(query, level, timestamp, formattedCaller, message, extraFields)
	if err != nil {
		fmt.Println("Error inserting log into DB:", err)
	}

	return len(p), nil
}
