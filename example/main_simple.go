package main

import (
	"embed"
	"log"

	"github.com/gostratum/dbx"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// User represents a user model
type User struct {
	ID    uint   `json:"id" gorm:"primaryKey"`
	Name  string `json:"name"`
	Email string `json:"email" gorm:"uniqueIndex"`
}

// Product represents a product model
type Product struct {
	ID    uint    `json:"id" gorm:"primaryKey"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Setup configuration for SQLite (easier for examples)
	v := viper.New()
	v.SetConfigType("yaml")
	v.Set("db.default", "primary")
	v.Set("db.databases.primary.driver", "sqlite")
	v.Set("db.databases.primary.dsn", "file::memory:?cache=shared")
	v.Set("db.databases.primary.log_level", "info")

	// Load and validate config
	config, err := dbx.LoadConfig(v)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger.Info("DBX Configuration loaded successfully")
	logger.Info("Default database", zap.String("default", config.Default))

	for name, dbConfig := range config.Databases {
		logger.Info("Database configured",
			zap.String("name", name),
			zap.String("driver", dbConfig.Driver),
			zap.String("log_level", dbConfig.LogLevel),
		)
	}

	// Test config validation
	if err := config.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	logger.Info("Configuration validation passed")

	// Test getting default database config
	defaultDB, err := config.GetDefaultDatabase()
	if err != nil {
		log.Fatalf("Failed to get default database: %v", err)
	}

	logger.Info("Default database config retrieved",
		zap.String("driver", defaultDB.Driver),
		zap.Int("max_open_conns", defaultDB.MaxOpenConns),
	)

	// This example shows the basic dbx configuration and validation
	// In a real application, you would use fx.App to wire everything together
	logger.Info("DBX module example completed successfully")
}
