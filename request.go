package main

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Sql          string
	CreatedAt    time.Time
	CompletedAt  time.Time
	Duration     int
	ConnectionID uuid.UUID
	conn         net.Conn
	connID       uint64
}

func (r Request) String() string {
	return fmt.Sprintf("ID: %v, UserID: %v, SQL: %v, CreatedAt: %v, CompletedAt: %v, Duration: %v, ConnectionID: %v", r.ID, r.Sql, r.Sql, r.CreatedAt, r.CompletedAt, r.Duration, r.ConnectionID)
}
