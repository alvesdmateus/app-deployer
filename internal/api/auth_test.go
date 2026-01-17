package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/alvesdmateus/app-deployer/pkg/config"
	"github.com/glebarez/sqlite"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAuthTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Auto-migrate models
	err = db.AutoMigrate(&state.User{}, &state.APIKey{})
	require.NoError(t, err)

	return db
}

func setupAuthHandler(db *gorm.DB) *AuthHandler {
	cfg := &config.AuthConfig{
		JWTSecret:          "test-secret-key-for-testing",
		JWTExpirationHours: 24,
		Enabled:            true,
	}
	return NewAuthHandler(db, cfg)
}

func TestRegister(t *testing.T) {
	db := setupAuthTestDB(t)
	handler := setupAuthHandler(db)

	t.Run("successful registration", func(t *testing.T) {
		body := RegisterRequest{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Register(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response AuthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotEmpty(t, response.Token)
		assert.Equal(t, "test@example.com", response.User.Email)
		assert.Equal(t, "Test User", response.User.Name)
		assert.Equal(t, "user", response.User.Role)
	})

	t.Run("duplicate email", func(t *testing.T) {
		body := RegisterRequest{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User 2",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Register(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("missing email", func(t *testing.T) {
		body := RegisterRequest{
			Password: "password123",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Register(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("password too short", func(t *testing.T) {
		body := RegisterRequest{
			Email:    "short@example.com",
			Password: "short",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Register(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLogin(t *testing.T) {
	db := setupAuthTestDB(t)
	handler := setupAuthHandler(db)

	// Create a test user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := state.User{
		ID:           uuid.New(),
		Email:        "login@example.com",
		PasswordHash: string(hashedPassword),
		Name:         "Login User",
		Role:         "user",
		Active:       true,
	}
	db.Create(&testUser)

	t.Run("successful login", func(t *testing.T) {
		body := LoginRequest{
			Email:    "login@example.com",
			Password: "password123",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response AuthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.NotEmpty(t, response.Token)
		assert.Equal(t, "login@example.com", response.User.Email)
	})

	t.Run("wrong password", func(t *testing.T) {
		body := LoginRequest{
			Email:    "login@example.com",
			Password: "wrongpassword",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("user not found", func(t *testing.T) {
		body := LoginRequest{
			Email:    "nonexistent@example.com",
			Password: "password123",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("disabled user", func(t *testing.T) {
		// Create disabled user
		hashedPw, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		disabledUser := state.User{
			ID:           uuid.New(),
			Email:        "disabled@example.com",
			PasswordHash: string(hashedPw),
			Name:         "Disabled User",
			Role:         "user",
			Active:       true, // Create as active first
		}
		db.Create(&disabledUser)
		// Then disable the user explicitly to bypass GORM default
		db.Model(&disabledUser).Update("active", false)

		body := LoginRequest{
			Email:    "disabled@example.com",
			Password: "password123",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestJWTAuthMiddleware(t *testing.T) {
	db := setupAuthTestDB(t)
	cfg := &config.AuthConfig{
		JWTSecret:          "test-secret-key-for-testing",
		JWTExpirationHours: 24,
		Enabled:            true,
	}
	handler := NewAuthHandler(db, cfg)

	// Create a test user and get a token
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := state.User{
		ID:           uuid.New(),
		Email:        "jwt@example.com",
		PasswordHash: string(hashedPassword),
		Name:         "JWT User",
		Role:         "user",
		Active:       true,
	}
	db.Create(&testUser)

	token, _, _ := handler.generateToken(&testUser)

	// Create a protected handler
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user != nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(user.Email))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})

	middleware := JWTAuthMiddleware(db, cfg)

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		middleware(protectedHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "jwt@example.com", rec.Body.String())
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		middleware(protectedHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		middleware(protectedHandler).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("auth disabled", func(t *testing.T) {
		disabledCfg := &config.AuthConfig{
			Enabled: false,
		}
		disabledMiddleware := JWTAuthMiddleware(db, disabledCfg)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		// Should pass through without authentication
		disabledMiddleware(protectedHandler).ServeHTTP(rec, req)

		// When auth is disabled, user won't be in context but request proceeds
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAPIKeyAuth(t *testing.T) {
	db := setupAuthTestDB(t)
	cfg := &config.AuthConfig{
		JWTSecret:          "test-secret-key-for-testing",
		JWTExpirationHours: 24,
		Enabled:            true,
	}
	handler := NewAuthHandler(db, cfg)

	// Create a test user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := state.User{
		ID:           uuid.New(),
		Email:        "apikey@example.com",
		PasswordHash: string(hashedPassword),
		Name:         "API Key User",
		Role:         "user",
		Active:       true,
	}
	db.Create(&testUser)

	// Get a token for creating API key
	token, _, _ := handler.generateToken(&testUser)

	t.Run("create and use API key", func(t *testing.T) {
		// Create API key
		body := CreateAPIKeyRequest{
			Name: "Test Key",
		}
		bodyBytes, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/api-keys", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		// Add user to context
		router := chi.NewRouter()
		router.Use(JWTAuthMiddleware(db, cfg))
		router.Post("/api/v1/auth/api-keys", handler.CreateAPIKey)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var createResp CreateAPIKeyResponse
		err := json.Unmarshal(rec.Body.Bytes(), &createResp)
		require.NoError(t, err)
		assert.NotEmpty(t, createResp.APIKey)
		assert.True(t, len(createResp.APIKey) > 0)

		// Use the API key
		middleware := JWTAuthMiddleware(db, cfg)
		protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			if user != nil {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(user.Email))
			} else {
				w.WriteHeader(http.StatusUnauthorized)
			}
		})

		req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req2.Header.Set("Authorization", "ApiKey "+createResp.APIKey)
		rec2 := httptest.NewRecorder()

		middleware(protectedHandler).ServeHTTP(rec2, req2)

		assert.Equal(t, http.StatusOK, rec2.Code)
		assert.Equal(t, "apikey@example.com", rec2.Body.String())
	})
}

func TestAdminOnlyMiddleware(t *testing.T) {
	db := setupAuthTestDB(t)
	cfg := &config.AuthConfig{
		JWTSecret:          "test-secret-key-for-testing",
		JWTExpirationHours: 24,
		Enabled:            true,
	}
	handler := NewAuthHandler(db, cfg)

	// Create admin user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	adminUser := state.User{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: string(hashedPassword),
		Name:         "Admin User",
		Role:         "admin",
		Active:       true,
	}
	db.Create(&adminUser)

	// Create regular user
	regularUser := state.User{
		ID:           uuid.New(),
		Email:        "regular@example.com",
		PasswordHash: string(hashedPassword),
		Name:         "Regular User",
		Role:         "user",
		Active:       true,
	}
	db.Create(&regularUser)

	adminToken, _, _ := handler.generateToken(&adminUser)
	userToken, _, _ := handler.generateToken(&regularUser)

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("admin access granted"))
	})

	router := chi.NewRouter()
	router.Route("/admin", func(r chi.Router) {
		r.Use(JWTAuthMiddleware(db, cfg))
		r.Use(AdminOnlyMiddleware)
		r.Get("/", adminHandler)
	})

	t.Run("admin user allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "admin access granted", rec.Body.String())
	})

	t.Run("regular user denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}
