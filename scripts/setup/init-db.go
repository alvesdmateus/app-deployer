package main

import (
	"fmt"
	"os"

	"github.com/alvesdmateus/app-deployer/internal/state"
	"github.com/alvesdmateus/app-deployer/pkg/config"
	"github.com/alvesdmateus/app-deployer/pkg/database"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Initialize logger
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	log.Info().Msg("Starting database initialization...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

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

	// Run migrations
	models := []interface{}{
		&state.Deployment{},
		&state.Infrastructure{},
		&state.Build{},
	}

	if err := database.Migrate(db, models...); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Close database connection
	if err := database.Close(db); err != nil {
		log.Error().Err(err).Msg("Failed to close database connection")
	}

	fmt.Println("\n✅ Database initialized successfully!")
	fmt.Println("\nCreated tables:")
	fmt.Println("  - deployments")
	fmt.Println("  - infrastructures")
	fmt.Println("  - builds")
}
