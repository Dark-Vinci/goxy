package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"

	"thesis/store"
)

// handleSignup creates a new user (admin-only)
func (p *Proxy) handleSignup(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	if err = json.NewDecoder(r.Body).Decode(&user); err != nil {
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

	u := store.User{
		ID:        uuid.New(),
		Username:  user.Username,
		Password:  string(hashedPassword),
		IsAdmin:   false,
		Role:      user.Role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
	}

	// Insert user
	_, err = p.store.userStore.Create(ctx, requestID, u)
	if err != nil {
		//todo; update to gorm error
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

func (p *Proxy) handleGetUser(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()
	userID := mux.Vars(r)["id"]

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		p.logger.Warn().Err(err).Msg("Invalid user ID")
		http.Error(w, "Invalid or missing userID", http.StatusBadRequest)
		return
	}

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	user, err := p.store.userStore.GetByID(ctx, requestID, userUUID)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get user")
		http.Error(w, "user not found", http.StatusNotFound)
	}

	p.logger.Info().Msgf("successfuly fetched user")

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(user)
}

// handleFetchUsers fetches all users (admin-only)
func (p *Proxy) handleFetchUsers(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	query := r.URL.Query()

	pageSizeStr := query.Get("page_size")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10 // default
	}

	// Extract and parse page
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // default
	}

	result, err := p.store.userStore.GetPaginatedUsers(ctx, requestID, page, pageSize)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get users")
		http.Error(w, "something went wrong", http.StatusNotFound)
		return
	}

	p.logger.Info().Msgf("successfuly fetched users")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(result)
}

// handleSignup creates a new user (admin-only)
func (p *Proxy) handleGetFailedHealthChecks(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	query := r.URL.Query()

	pageSizeStr := query.Get("page_size")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10 // default
	}

	// Extract and parse page
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // default
	}

	result, err := p.store.healthCheckStore.GetFailedHealthChecks(ctx, requestID, page, pageSize)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get health checks")
		http.Error(w, "something went wrong", http.StatusNotFound)
		return
	}

	p.logger.Info().Msgf("successfuly fetched failed health checks")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(result)
}

// handleGetHealthChecks get health checks (admin-only)
func (p *Proxy) handleGetHealthChecks(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	query := r.URL.Query()

	pageSizeStr := query.Get("page_size")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10 // default
	}

	// Extract and parse page
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // default
	}

	result, err := p.store.healthCheckStore.GetPaginatedHealthChecks(ctx, requestID, page, pageSize)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get health checks")
		http.Error(w, "something went wrong", http.StatusNotFound)
		return
	}

	p.logger.Info().Msgf("successfuly fetched health checks")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(result)
}

func (p *Proxy) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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
	fetched, err := p.store.userStore.GetByUsername(ctx, requestID, user.Username)
	if err != nil {
		p.logger.Error().Err(err).Msgf("Failed to check user %s existence", user.Username)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			p.logger.Error().Err(err).Msg("Failed to hash password")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		fetched.Password = string(hashedPassword)
	}

	if user.Role != "" {
		fetched.Role = user.Role
	}

	// Update user
	err = p.store.userStore.Update(ctx, requestID, *fetched)
	if err != nil {
		p.logger.Error().Err(err).Msgf("Failed to update user %s", user.Username)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	p.logger.Info().Msgf("User %s updated by %s", user.Username, username)

	_ = json.NewEncoder(w).Encode(map[string]string{"message": "User updated successfully"})
}

func (p *Proxy) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	var credential struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to decode login request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify credentials
	fetched, err := p.store.userStore.GetByUsername(ctx, requestID, credential.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			p.logger.Warn().Msgf("User not found: %s", credential.Username)
		} else {
			p.logger.Warn().Err(err).Msgf("Failed to query user %s", credential.Username)
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(fetched.Password), []byte(credential.Password)); err != nil {
		p.logger.Warn().Msgf("Invalid password for %s", credential.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": credential.Username,
		"role":     fetched.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(p.config.JWTSecret))
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to generate JWT")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	p.logger.Info().Msgf("User %s logged in with role %s", credential.Username, fetched.Role)

	_ = json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

// handleGetLogs fetches logs from the DB
func (p *Proxy) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	query := r.URL.Query()

	pageSizeStr := query.Get("page_size")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10 // default
	}

	// Extract and parse page
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // default
	}

	filter := query.Get("filter")

	result, err := p.store.logsStore.GetPaginatedLogs(ctx, requestID, page, pageSize, filter)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get logs")
		http.Error(w, "something went wrong", http.StatusServiceUnavailable)
		return
	}

	p.logger.Info().Msgf("successfuly fetched logs")

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (p *Proxy) handleGetLogsByRequestID(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	// Validate JWT and ensure admin role
	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	requestRequestIDStr := mux.Vars(r)["request_id"]

	requestRequestID, err := uuid.Parse(requestRequestIDStr)
	if err != nil {
		p.logger.Warn().Err(err).Msgf("Invalid request ID %s", requestRequestIDStr)
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
	}

	result, err := p.store.logsStore.GetRequestIDLogs(ctx, requestID, requestRequestID)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get logs")
		http.Error(w, "something went wrong", http.StatusServiceUnavailable)
		return
	}

	p.logger.Info().Msgf("successfuly fetched logs")

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (p *Proxy) handleGetDBRequestByID(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	requestIDStr := mux.Vars(r)["id"]
	requestRequestID, err := uuid.Parse(requestIDStr)
	if err != nil {
		p.logger.Warn().Err(err).Msgf("Invalid request ID %s", requestIDStr)
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
	}

	result, err := p.store.requestStore.GetByRequestID(ctx, requestID, requestRequestID)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get logs")
		http.Error(w, "something went wrong", http.StatusServiceUnavailable)
		return
	}

	p.logger.Info().Msgf("successfuly fetched logs")

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (p *Proxy) handleGetDBRequest(w http.ResponseWriter, r *http.Request) {
	ctx, requestID := r.Context(), uuid.New()

	username, role, err := p.validateJWTFromHeader(ctx, requestID, r)
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

	query := r.URL.Query()

	pageSizeStr := query.Get("page_size")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10 // default
	}

	// Extract and parse page
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1 // default
	}

	result, err := p.store.requestStore.GetPaginatedRequest(ctx, requestID, page, pageSize)
	if err != nil {
		p.logger.Error().Err(err).Msg("Failed to get logs")
		http.Error(w, "something went wrong", http.StatusServiceUnavailable)
		return
	}

	p.logger.Info().Msgf("successfuly fetched logs")

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}
