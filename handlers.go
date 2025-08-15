package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// handleSignup creates a new user (admin-only)
func (p *Proxy) handleSignup(w http.ResponseWriter, r *http.Request) {
	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(r)
	if err != nil {
		p.logger.Warn().Err(err).Msg("Invalid or missing token for signup")
		http.Error(w, "Invalid or missing token", http.StatusUnauthorized)
		return
	}

	if role != UserRoleAdmin {
		p.logger.Warn().Msgf("User %s with role %s attempted signup", username, role)
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	var user struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to decode signup request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate input
	if user.Username == "" || user.Password == "" || user.Role == "" {
		p.logger.Warn().Msg("Missing required fields in signup request")
		http.Error(w, "Username, password, and role are required", http.StatusBadRequest)
		return
	}

	if !isValidRole(UserRole(user.Role)) {
		p.logger.Warn().Msgf("Invalid role %s in signup request", user.Role)
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to hash password")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Insert user
	_, err = p.sqliteDB.Exec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)",
		user.Username, hashedPassword, user.Role)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			p.logger.Warn().Msgf("Username %s already exists", user.Username)
			http.Error(w, "Username already exists", http.StatusConflict)
			return
		}
		p.logger.Error().Err(err).Msgf("Failed to insert user %s", user.Username)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	p.logger.Info().Msgf("User %s created with role %s by %s", user.Username, user.Role, username)
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(map[string]string{"message": "User created successfully"})
}

func (p *Proxy) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(r)
	if err != nil {
		p.logger.Warn().Err(err).Msg("Invalid or missing token for update-user")
		http.Error(w, "Invalid or missing token", http.StatusUnauthorized)
		return
	}

	if role != UserRoleAdmin {
		p.logger.Warn().Msgf("User %s with role %s attempted update-user", username, role)
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	var user struct {
		Username string `json:"username"`
		Password string `json:"password,omitempty"`
		Role     string `json:"role,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to decode update-user request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate input
	if user.Username == "" {
		p.logger.Warn().Msg("Missing username in update-user request")
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	if user.Password == "" && user.Role == "" {
		p.logger.Warn().Msg("No fields to update in update-user request")
		http.Error(w, "At least one of password or role must be provided", http.StatusBadRequest)
		return
	}

	if user.Role != "" && !isValidRole(UserRole(user.Role)) {
		p.logger.Warn().Msgf("Invalid role %s in update-user request", user.Role)
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	// Check if user exists
	var exists bool
	err = p.sqliteDB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", user.Username).Scan(&exists)
	if err != nil {
		p.logger.Error().Err(err).Msgf("Failed to check user %s existence", user.Username)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !exists {
		p.logger.Warn().Msgf("User %s not found for update", user.Username)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Prepare update query
	query := "UPDATE users SET"
	args := []interface{}{}
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			p.logger.Error().Err(err).Msg("Failed to hash password")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		query += " password = ?"
		args = append(args, hashedPassword)
	}

	if user.Role != "" {
		if len(args) > 0 {
			query += ","
		}
		query += " role = ?"
		args = append(args, user.Role)
	}

	query += " WHERE username = ?"
	args = append(args, user.Username)

	// Update user
	_, err = p.sqliteDB.Exec(query, args...)
	if err != nil {
		p.logger.Error().Err(err).Msgf("Failed to update user %s", user.Username)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	p.logger.Info().Msgf("User %s updated by %s", user.Username, username)

	_ = json.NewEncoder(w).Encode(map[string]string{"message": "User updated successfully"})
}

func (p *Proxy) handleLogin(w http.ResponseWriter, r *http.Request) {
	var credential creds

	if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to decode login request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify credentials
	var storedHash, role string
	err := p.sqliteDB.QueryRow("SELECT password, role FROM users WHERE username = ?", credential.Username).Scan(&storedHash, &role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			p.logger.Warn().Msgf("User not found: %s", credential.Username)
		} else {
			p.logger.Warn().Err(err).Msgf("Failed to query user %s", credential.Username)
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(credential.Password)); err != nil {
		p.logger.Warn().Msgf("Invalid password for %s", credential.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": credential.Username,
		"role":     role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(p.config.JWTSecret))
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to generate JWT")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	p.logger.Info().Msgf("User %s logged in with role %s", credential.Username, role)

	_ = json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}
