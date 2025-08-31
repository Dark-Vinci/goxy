package main

import (
	"fmt"
	"net"
	"time"
)

func pingPostgres(conn net.Conn) error {
	// Set a timeout for the ping operation
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("failed to set deadline: %w", err)
	}

	// Simple query message to ping the database
	// PostgreSQL wire protocol: 'Q' (Query) message followed by "SELECT 1;\0"
	query := "Q\x00\x00\x00\x0fSELECT 1;\x00"

	// Send the query
	_, err := conn.Write([]byte(query))
	if err != nil {
		return fmt.Errorf("failed to write ping query: %w", err)
	}

	// Read response to verify connection is working
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read ping response: %w", err)
	}

	// Clear the deadline
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("failed to clear deadline: %w", err)
	}

	return nil
}
