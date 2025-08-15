package main

import (
	"os"
	"strconv"
	"strings"
)

// Config holds proxy configuration
type Config struct {
	listenAddr    string
	master        string
	pingInterval  int
	slaves        []string
	HTTPListen    string
	JWTSecret     string
	adminUser     string
	adminPassword string
}

func NewConfig() *Config {
	master := os.Getenv("MASTER")
	slavesStr := os.Getenv("SLAVES")
	listenAddr := os.Getenv("LISTEN_ADDRESS")
	pingInterval := os.Getenv("PING_INTERVAL")

	ping, _ := strconv.Atoi(pingInterval)

	slaves := strings.Split(slavesStr, ",")

	return &Config{
		master:       master,
		slaves:       slaves,
		listenAddr:   listenAddr,
		pingInterval: ping,
	}
}
