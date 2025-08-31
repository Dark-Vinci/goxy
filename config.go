package main

import (
	"os"
	"strconv"
	"strings"
)

// Config holds proxy configuration
type Config struct {
	listenAddr         string
	pingInterval       int
	servers            []string
	HTTPListen         string
	JWTSecret          string
	adminUser          string
	adminPassword      string
	connectionPoolSize int
}

func NewConfig() *Config {
	slavesStr := os.Getenv("SLAVES")
	listenAddr := os.Getenv("LISTEN_ADDRESS")
	pingInterval := os.Getenv("PING_INTERVAL")
	httpListen := os.Getenv("HTTP_LISTENER")
	jwtSecret := os.Getenv("JWT_SECRET")
	adminUser := os.Getenv("ADMIN_USER")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	connectionPoolSize := os.Getenv("CONNECTION_POOL_SIZE")

	pingIntInterval, _ := strconv.Atoi(pingInterval)

	connectionPoolSizeInt, err := strconv.Atoi(connectionPoolSize)
	if err != nil {
		connectionPoolSizeInt = 10
	}

	slaves := strings.Split(slavesStr, ",")

	return &Config{
		servers:            slaves,
		listenAddr:         listenAddr,
		pingInterval:       pingIntInterval,
		HTTPListen:         httpListen,
		JWTSecret:          jwtSecret,
		adminUser:          adminUser,
		adminPassword:      adminPassword,
		connectionPoolSize: connectionPoolSizeInt,
	}
}
