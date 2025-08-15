package main

// Config holds proxy configuration
type Config struct {
	listenAddr string
	master     string
	slaves     []string
}
