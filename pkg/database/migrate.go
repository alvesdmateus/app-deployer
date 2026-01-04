package database

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Migrate runs database migrations for all models
func Migrate(db *gorm.DB, models ...interface{}) error {
	log.Info().Msg("Running database migrations...")

	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info().Int("models", len(models)).Msg("Database migrations completed successfully")
	return nil
}

// DropAllTables drops all tables (use with caution - for testing only)
func DropAllTables(db *gorm.DB, models ...interface{}) error {
	log.Warn().Msg("Dropping all database tables...")

	for _, model := range models {
		if err := db.Migrator().DropTable(model); err != nil {
			return fmt.Errorf("failed to drop table: %w", err)
		}
	}

	log.Info().Msg("All tables dropped successfully")
	return nil
}

// HasTable checks if a table exists
func HasTable(db *gorm.DB, model interface{}) bool {
	return db.Migrator().HasTable(model)
}
