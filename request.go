package main

import (
	"fmt"
	"github.com/google/uuid"
	"net"
)

type Request struct {
	connID    uint64
	query     string
	requestId uuid.UUID
	role      string
	userID    uuid.UUID
	conn      net.Conn
}

func (r *Request) String() string {
	return fmt.Sprintf("[Conn %d] %s", r.connID, r.query)
}
