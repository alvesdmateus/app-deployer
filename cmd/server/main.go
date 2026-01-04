package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mateus/app-deployer/internal/api"
	"github.com/mateus/app-deployer/internal/state"
	"github.com/mateus/app-deployer/pkg/config"
	"github.com/mateus/app-deployer/pkg/database"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Initialize logger
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Set log level
	setLogLevel(cfg.Server.LogLevel)

	log.Info().
		Str("app", "app-deployer").
		Str("port", cfg.Server.Port).
		Msg("Starting application")

	// Connect to database
	dbConfig := database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		DBName:          cfg.Database.DBName,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer func() {
		if err := database.Close(db); err != nil {
			log.Error().Err(err).Msg("Failed to close database")
		}
	}()

	// Run migrations
	models := []interface{}{
		&state.Deployment{},
		&state.Infrastructure{},
		&state.Build{},
	}

	if err := database.Migrate(db, models...); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Perform health check
	if err := database.HealthCheck(db); err != nil {
		log.Fatal().Err(err).Msg("Database health check failed")
	}

	log.Info().Msg("Database is healthy")

	// Initialize HTTP server
	apiServer := api.NewServer(db)
	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      apiServer.Handler(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Info().
			Str("port", cfg.Server.Port).
			Msg("Starting HTTP server")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	log.Info().
		Str("port", cfg.Server.Port).
		Msg("Application ready - HTTP server running")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down application...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown failed")
	}

	log.Info().Msg("Application stopped")
}

// setLogLevel sets the global log level based on configuration
func setLogLevel(level string) {
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
