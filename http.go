package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

// HTTPServer starts the HTTP server for login
func (p *Proxy) HTTPServer() error {
	r := mux.NewRouter()

	//Health check
	r.HandleFunc("/health/healthy", p.handleSignup).Methods("GET")
	r.HandleFunc("/health/unhealthy", p.handleLogin).Methods("GET")

	// Users
	r.HandleFunc("/users/login", p.handleLogin).Methods("POST")
	r.HandleFunc("/users/signup", p.handleSignup).Methods("POST")
	r.HandleFunc("/users/update-user", p.handleUpdateUser).Methods("PUT")

	//Logs
	r.HandleFunc("/logs", p.handleSignup).Methods("GET")
	r.HandleFunc("/logs/{request_id}", p.handleLogin).Methods("GET")

	// Requests
	r.HandleFunc("/request", p.handleSignup).Methods("GET")
	r.HandleFunc("/request/{id}", p.handleLogin).Methods("GET")

	p.logger.Info().Msgf("HTTP server listening on %s", p.config.HTTPListen)

	return http.ListenAndServe(p.config.HTTPListen, r)
}

// isValidRole checks if a role is valid
func isValidRole(role UserRole) bool {
	return role == UserRoleAdmin || role == UserRoleReadWrite || role == UserRoleReadOnly
}

// validateJWT validates the JWT and returns username and role
func (p *Proxy) validateJWT(tokenString string) (string, UserRole, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(p.config.JWTSecret), nil
	})
	if err != nil {
		return "", "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username, _ := claims["username"].(string)
		roleStr, _ := claims["role"].(string)
		// Verify user exists in SQLite
		var storedRole string
		err = p.sqliteDB.QueryRow("SELECT role FROM users WHERE username = ?", username).Scan(&storedRole)
		if err != nil {
			return "", "", fmt.Errorf("user %s not found: %w", username, err)
		}
		if storedRole != roleStr {
			return "", "", fmt.Errorf("role mismatch for %s", username)
		}
		return username, UserRole(roleStr), nil
	}

	return "", "", fmt.Errorf("invalid token claims")
}

// validateJWTFromHeader validates the JWT from the Authorization header
func (p *Proxy) validateJWTFromHeader(r *http.Request) (string, UserRole, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", "", fmt.Errorf("missing or invalid Authorization header")
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	return p.validateJWT(tokenString)
}
