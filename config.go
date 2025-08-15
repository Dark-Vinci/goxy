package main

import (
	"os"
	"strings"
)

// Config holds proxy configuration
type Config struct {
	listenAddr string
	master     string
	slaves     []string
}

func NewConfig() *Config {
	master := os.Getenv("MASTER")
	slavesStr := os.Getenv("SLAVES")
	listenAddr := os.Getenv("LISTEN_ADDRESS")
	
	slaves := strings.Split(slavesStr, ",")

	return &Config{
		master:     master,
		slaves:     slaves,
		listenAddr: listenAddr,
	}
}
