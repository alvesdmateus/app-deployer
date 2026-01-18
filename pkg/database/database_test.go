package database

import (
	"os"
	"testing"
	"time"
)

// getTestConfig returns a Config for testing
func getTestConfig() Config {
	// Allow override via environment variable
	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		host = "localhost"
	}

	return Config{
		Host:            host,
		Port:            5432,
		User:            "deployer",
		Password:        "test_password",
		DBName:          "app_deployer_test",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// TestNew tests database connection creation
func TestNew(t *testing.T) {
	config := getTestConfig()

	db, err := New(config)
	if err != nil {
		t.Skipf("Skipping test - PostgreSQL not available: %v", err)
	}
	defer Close(db)

	if db == nil {
		t.Fatal("Expected database connection, got nil")
	}
}

// TestHealthCheck tests database health check
func TestHealthCheck(t *testing.T) {
	config := getTestConfig()

	db, err := New(config)
	if err != nil {
		t.Skipf("Skipping test - PostgreSQL not available: %v", err)
	}
	defer Close(db)

	// Test health check
	if err := HealthCheck(db); err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

// TestClose tests database connection closure
func TestClose(t *testing.T) {
	config := getTestConfig()

	db, err := New(config)
	if err != nil {
		t.Skipf("Skipping test - PostgreSQL not available: %v", err)
	}

	// Test close
	if err := Close(db); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestConnectionPool tests connection pool configuration
func TestConnectionPool(t *testing.T) {
	config := getTestConfig()
	config.MaxOpenConns = 25
	config.MaxIdleConns = 10

	db, err := New(config)
	if err != nil {
		t.Skipf("Skipping test - PostgreSQL not available: %v", err)
	}
	defer Close(db)

	// Get underlying SQL database
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get database instance: %v", err)
	}

	// Verify connection pool settings
	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != config.MaxOpenConns {
		t.Errorf("Expected MaxOpenConns %d, got %d", config.MaxOpenConns, stats.MaxOpenConnections)
	}
}
