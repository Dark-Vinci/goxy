package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// validateJWT validates the JWT and returns username and role
func (p *Proxy) validateJWT(ctx context.Context, requestID uuid.UUID, tokenString string) (string, UserRole, error) {
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
		user, err := p.store.userStore.GetByUsername(ctx, requestID, username)

		if err != nil {
			return "", "", fmt.Errorf("user %s not found: %w", username, err)
		}

		if user.Role != roleStr {
			return "", "", fmt.Errorf("role mismatch for %s", username)
		}

		return username, UserRole(roleStr), nil
	}

	return "", "", fmt.Errorf("invalid token claims")
}

// validateJWTFromHeader validates the JWT from the Authorization header
func (p *Proxy) validateJWTFromHeader(ctx context.Context, requestID uuid.UUID, r *http.Request) (string, UserRole, error) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", "", fmt.Errorf("missing or invalid Authorization header")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	return p.validateJWT(ctx, requestID, tokenString)
}

// isValidRole checks if a role is valid
func isValidRole(role UserRole) bool {
	return role == UserRoleAdmin || role == UserRoleReadWrite || role == UserRoleReadOnly
}
