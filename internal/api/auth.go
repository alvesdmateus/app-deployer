package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/alvesdmateus/app-deployer/pkg/config"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Context keys for auth
type contextKey string

const (
	UserContextKey   contextKey = "user"
	ClaimsContextKey contextKey = "claims"
)

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	db        *gorm.DB
	jwtSecret []byte
	jwtExpiry time.Duration
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *gorm.DB, cfg *config.AuthConfig) *AuthHandler {
	return &AuthHandler{
		db:        db,
		jwtSecret: []byte(cfg.JWTSecret),
		jwtExpiry: time.Duration(cfg.JWTExpirationHours) * time.Hour,
	}
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	Token     string       `json:"token"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

// UserResponse represents a user in API responses
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKeyResponse represents an API key in API responses
type APIKeyResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateAPIKeyRequest represents the request to create an API key
type CreateAPIKeyRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes,omitempty"`
	ExpiresIn *int     `json:"expires_in_days,omitempty"` // Days until expiration
}

// CreateAPIKeyResponse includes the full key (only shown once)
type CreateAPIKeyResponse struct {
	APIKey string         `json:"api_key"` // Full key, only shown once
	Key    APIKeyResponse `json:"key"`
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := DecodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		RespondWithError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	if len(req.Password) < 8 {
		RespondWithError(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	// Check if user already exists
	var existingUser state.User
	if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		RespondWithError(w, http.StatusConflict, "User already exists")
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Create user
	user := state.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Name:         req.Name,
		Role:         "user",
		Active:       true,
	}

	if err := h.db.Create(&user).Error; err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Generate token
	token, expiresAt, err := h.generateToken(&user)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		RespondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response := AuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      userToResponse(&user),
	}

	RespondWithJSON(w, http.StatusCreated, response)
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := DecodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		RespondWithError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	// Find user
	var user state.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Check if user is active
	if !user.Active {
		RespondWithError(w, http.StatusUnauthorized, "Account is disabled")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Generate token
	token, expiresAt, err := h.generateToken(&user)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		RespondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response := AuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      userToResponse(&user),
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// GetCurrentUser handles GET /api/v1/auth/me
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	RespondWithJSON(w, http.StatusOK, userToResponse(user))
}

// CreateAPIKey handles POST /api/v1/auth/api-keys
func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateAPIKeyRequest
	if err := DecodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		RespondWithError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Generate a random API key
	keyBytes := make([]byte, 32)
	keyID := uuid.New()
	copy(keyBytes, keyID[:])
	rawKey := "apd_" + hex.EncodeToString(keyBytes) // apd = app-deployer prefix

	// Hash the key for storage
	keyHash := sha256.Sum256([]byte(rawKey))
	keyHashHex := hex.EncodeToString(keyHash[:])

	// Set expiration if specified
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &exp
	}

	// Create API key record
	apiKey := state.APIKey{
		ID:        keyID,
		UserID:    user.ID,
		Name:      req.Name,
		KeyHash:   keyHashHex,
		KeyPrefix: rawKey[:12], // "apd_" + first 8 chars
		Scopes:    strings.Join(req.Scopes, ","),
		ExpiresAt: expiresAt,
		Active:    true,
	}

	if err := h.db.Create(&apiKey).Error; err != nil {
		log.Error().Err(err).Msg("Failed to create API key")
		RespondWithError(w, http.StatusInternalServerError, "Failed to create API key")
		return
	}

	response := CreateAPIKeyResponse{
		APIKey: rawKey, // Only time we return the full key
		Key:    apiKeyToResponse(&apiKey),
	}

	RespondWithJSON(w, http.StatusCreated, response)
}

// ListAPIKeys handles GET /api/v1/auth/api-keys
func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var apiKeys []state.APIKey
	if err := h.db.Where("user_id = ?", user.ID).Find(&apiKeys).Error; err != nil {
		log.Error().Err(err).Msg("Failed to list API keys")
		RespondWithError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}

	responses := make([]APIKeyResponse, len(apiKeys))
	for i, key := range apiKeys {
		responses[i] = apiKeyToResponse(&key)
	}

	RespondWithJSON(w, http.StatusOK, responses)
}

// RevokeAPIKey handles DELETE /api/v1/auth/api-keys/{id}
func (h *AuthHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	keyID := chi.URLParam(r, "id")
	keyUUID, err := uuid.Parse(keyID)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}

	// Find and verify ownership
	var apiKey state.APIKey
	if err := h.db.Where("id = ? AND user_id = ?", keyUUID, user.ID).First(&apiKey).Error; err != nil {
		RespondWithError(w, http.StatusNotFound, "API key not found")
		return
	}

	// Soft delete the key
	if err := h.db.Delete(&apiKey).Error; err != nil {
		log.Error().Err(err).Msg("Failed to revoke API key")
		RespondWithError(w, http.StatusInternalServerError, "Failed to revoke API key")
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]string{"message": "API key revoked"})
}

// generateToken creates a new JWT token for a user
func (h *AuthHandler) generateToken(user *state.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(h.jwtExpiry)

	claims := JWTClaims{
		UserID: user.ID.String(),
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "app-deployer",
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signedToken, expiresAt, nil
}

// JWTAuthMiddleware creates a middleware that validates JWT tokens
func JWTAuthMiddleware(db *gorm.DB, cfg *config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				RespondWithError(w, http.StatusUnauthorized, "Authorization header required")
				return
			}

			// Check for Bearer token or API key
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString := strings.TrimPrefix(authHeader, "Bearer ")
				user, claims, err := validateJWT(db, tokenString, cfg.JWTSecret)
				if err != nil {
					log.Debug().Err(err).Msg("JWT validation failed")
					RespondWithError(w, http.StatusUnauthorized, "Invalid token")
					return
				}

				// Add user and claims to context
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				ctx = context.WithValue(ctx, ClaimsContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if strings.HasPrefix(authHeader, "ApiKey ") {
				apiKey := strings.TrimPrefix(authHeader, "ApiKey ")
				user, err := validateAPIKey(db, apiKey)
				if err != nil {
					log.Debug().Err(err).Msg("API key validation failed")
					RespondWithError(w, http.StatusUnauthorized, "Invalid API key")
					return
				}

				// Add user to context
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			RespondWithError(w, http.StatusUnauthorized, "Invalid authorization format")
		})
	}
}

// OptionalAuthMiddleware validates auth if provided but doesn't require it
func OptionalAuthMiddleware(db *gorm.DB, cfg *config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check for Bearer token
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString := strings.TrimPrefix(authHeader, "Bearer ")
				user, claims, err := validateJWT(db, tokenString, cfg.JWTSecret)
				if err == nil {
					ctx := context.WithValue(r.Context(), UserContextKey, user)
					ctx = context.WithValue(ctx, ClaimsContextKey, claims)
					r = r.WithContext(ctx)
				}
			} else if strings.HasPrefix(authHeader, "ApiKey ") {
				apiKey := strings.TrimPrefix(authHeader, "ApiKey ")
				user, err := validateAPIKey(db, apiKey)
				if err == nil {
					ctx := context.WithValue(r.Context(), UserContextKey, user)
					r = r.WithContext(ctx)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnlyMiddleware requires the user to have admin role
func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}

		if user.Role != "admin" {
			RespondWithError(w, http.StatusForbidden, "Admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateJWT validates a JWT token and returns the associated user
func validateJWT(db *gorm.DB, tokenString string, secret string) (*state.User, *JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, nil, errors.New("invalid token claims")
	}

	// Get user from database
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, nil, errors.New("invalid user ID in token")
	}

	var user state.User
	if err := db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, nil, errors.New("user not found")
	}

	if !user.Active {
		return nil, nil, errors.New("user account is disabled")
	}

	return &user, claims, nil
}

// validateAPIKey validates an API key and returns the associated user
func validateAPIKey(db *gorm.DB, rawKey string) (*state.User, error) {
	// Hash the provided key
	keyHash := sha256.Sum256([]byte(rawKey))
	keyHashHex := hex.EncodeToString(keyHash[:])

	// Find the API key
	var apiKey state.APIKey
	if err := db.Where("key_hash = ? AND active = ?", keyHashHex, true).First(&apiKey).Error; err != nil {
		return nil, errors.New("API key not found")
	}

	// Check expiration
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("API key expired")
	}

	// Update last used
	now := time.Now()
	db.Model(&apiKey).Update("last_used", now)

	// Get the user
	var user state.User
	if err := db.First(&user, "id = ?", apiKey.UserID).Error; err != nil {
		return nil, errors.New("user not found")
	}

	if !user.Active {
		return nil, errors.New("user account is disabled")
	}

	return &user, nil
}

// GetUserFromContext retrieves the authenticated user from the request context
func GetUserFromContext(ctx context.Context) *state.User {
	user, ok := ctx.Value(UserContextKey).(*state.User)
	if !ok {
		return nil
	}
	return user
}

// GetClaimsFromContext retrieves the JWT claims from the request context
func GetClaimsFromContext(ctx context.Context) *JWTClaims {
	claims, ok := ctx.Value(ClaimsContextKey).(*JWTClaims)
	if !ok {
		return nil
	}
	return claims
}

// userToResponse converts a User model to a UserResponse
func userToResponse(user *state.User) UserResponse {
	return UserResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}
}

// apiKeyToResponse converts an APIKey model to an APIKeyResponse
func apiKeyToResponse(key *state.APIKey) APIKeyResponse {
	var scopes []string
	if key.Scopes != "" {
		scopes = strings.Split(key.Scopes, ",")
	}

	return APIKeyResponse{
		ID:        key.ID.String(),
		Name:      key.Name,
		KeyPrefix: key.KeyPrefix,
		Scopes:    scopes,
		ExpiresAt: key.ExpiresAt,
		LastUsed:  key.LastUsed,
		CreatedAt: key.CreatedAt,
	}
}
