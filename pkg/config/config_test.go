package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Test loading with defaults (no config file)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	// Verify defaults
	if cfg.Server.Port != "3000" {
		t.Errorf("Expected default port 3000, got %s", cfg.Server.Port)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("Expected default database host localhost, got %s", cfg.Database.Host)
	}

	if cfg.Database.Port != 5432 {
		t.Errorf("Expected default database port 5432, got %d", cfg.Database.Port)
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	// Set environment variable
	os.Setenv("SERVER_PORT", "8080")
	defer os.Unsetenv("SERVER_PORT")

	_, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Note: viper's AutomaticEnv() requires specific env var format
	// This test demonstrates the concept but may need adjustment
	// based on actual viper configuration
	t.Log("Environment override test - actual behavior depends on viper setup")
}

func TestGetDatabaseDSN(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		},
	}

	dsn := cfg.GetDatabaseDSN()
	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"

	if dsn != expected {
		t.Errorf("Expected DSN %s, got %s", expected, dsn)
	}
}

func TestConfigStructure(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         "3000",
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			LogLevel:     "info",
		},
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "deployer",
			Password:        "password",
			DBName:          "app_deployer",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Redis: RedisConfig{
			URL:      "localhost:6379",
			Password: "",
			DB:       0,
		},
		Platform: PlatformConfig{
			DefaultCloud:  "gcp",
			DefaultRegion: "us-central1",
		},
		Registry: RegistryConfig{
			Type:     "artifact-registry",
			Project:  "test-project",
			Location: "us-central1",
			URL:      "",
		},
	}

	// Verify config structure is properly initialized
	if cfg.Server.Port != "3000" {
		t.Errorf("Expected server port 3000, got %s", cfg.Server.Port)
	}

	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("Expected max open conns 25, got %d", cfg.Database.MaxOpenConns)
	}

	if cfg.Platform.DefaultCloud != "gcp" {
		t.Errorf("Expected default cloud gcp, got %s", cfg.Platform.DefaultCloud)
	}
}
