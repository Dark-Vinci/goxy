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
	httpListen := os.Getenv("HTTP_LISTENER")
	jwtSecret := os.Getenv("JWT_SECRET")
	adminUser := os.Getenv("ADMIN_USER")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	ping, _ := strconv.Atoi(pingInterval)

	slaves := strings.Split(slavesStr, ",")

	return &Config{
		master:        master,
		slaves:        slaves,
		listenAddr:    listenAddr,
		pingInterval:  ping,
		HTTPListen:    httpListen,
		JWTSecret:     jwtSecret,
		adminUser:     adminUser,
		adminPassword: adminPassword,
	}
}
