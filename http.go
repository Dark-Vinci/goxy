package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

// HTTPServer starts the HTTP server for login
func (p *Proxy) HTTPServer() error {
	r := mux.NewRouter()

	//Health check
	r.HandleFunc("/health/healthy", p.handleGetHealthChecks).Methods("GET")
	r.HandleFunc("/health/unhealthy", p.handleGetFailedHealthChecks).Methods("GET")

	// Users
	r.HandleFunc("/users/login", p.handleLogin).Methods("POST")
	r.HandleFunc("/users/signup", p.handleSignup).Methods("POST")
	r.HandleFunc("/users/update-user", p.handleUpdateUser).Methods("PUT")
	r.HandleFunc("/users", p.handleFetchUsers).Methods("GET")
	r.HandleFunc("/users/{id}", p.handleGetUser).Methods("GET")

	//Logs
	r.HandleFunc("/logs", p.handleGetLogs).Methods("GET")
	r.HandleFunc("/logs/{request_id}", p.handleGetLogsByRequestID).Methods("GET")

	// Requests
	r.HandleFunc("/request", p.handleGetDBRequest).Methods("GET")
	r.HandleFunc("/request/{id}", p.handleGetDBRequestByID).Methods("GET")

	p.logger.Info().Msgf("HTTP server listening on %s", p.config.HTTPListen)

	return http.ListenAndServe(p.config.HTTPListen, r)
}
