package database

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestNew tests database connection creation
func TestNew(t *testing.T) {
	config := Config{
		Host:            "localhost",
		Port:            5432,
		User:            "test",
		Password:        "test",
		DBName:          "test",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	// Note: This test would require a running PostgreSQL instance
	// For unit tests, we typically use mock databases or skip this test
	t.Skip("Skipping integration test - requires PostgreSQL instance")

	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database connection: %v", err)
	}

	if db == nil {
		t.Fatal("Expected database connection, got nil")
	}

	// Clean up
	if err := Close(db); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

// TestHealthCheck tests database health check
func TestHealthCheck(t *testing.T) {
	t.Skip("Skipping test - requires CGO for SQLite")
	// Create an in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Test health check
	if err := HealthCheck(db); err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

// TestClose tests database connection closure
func TestClose(t *testing.T) {
	t.Skip("Skipping test - requires CGO for SQLite")
	// Create an in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Test close
	if err := Close(db); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestConnectionPool tests connection pool configuration
func TestConnectionPool(t *testing.T) {
	config := Config{
		Host:            "localhost",
		Port:            5432,
		User:            "test",
		Password:        "test",
		DBName:          "test",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
	}

	t.Skip("Skipping integration test - requires PostgreSQL instance")

	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database connection: %v", err)
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
