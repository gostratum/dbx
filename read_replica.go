package dbx

import (
	"fmt"

	"github.com/gostratum/core/logx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// ReadReplicaConfig holds configuration for read replicas
type ReadReplicaConfig struct {
	// DSNs for read replicas
	DSNs []string
	// Load balancing policy: "random" or "round_robin"
	Policy string
}

// configureReadReplicas configures read replicas for a database connection
func configureReadReplicas(db *gorm.DB, replicas []string, logger logx.Logger) error {
	if len(replicas) == 0 {
		return nil
	}

	logger.Info("Configuring read replicas",
		logx.Int("count", len(replicas)),
	)

	// Create replica dialectors
	replicaDialectors := make([]gorm.Dialector, len(replicas))
	for i, dsn := range replicas {
		replicaDialectors[i] = postgres.Open(dsn)
		logger.Debug("Added read replica",
			logx.Int("index", i),
			logx.String("dsn", sanitizeDSN(dsn)),
		)
	}

	// Configure dbresolver plugin
	err := db.Use(dbresolver.Register(dbresolver.Config{
		Replicas: replicaDialectors,
		// Use read replicas for SELECT queries
		Policy: dbresolver.RandomPolicy{}, // Can be changed to RoundRobinPolicy
	}).
		// Set connection pool for replicas
		SetConnMaxIdleTime(DefaultDatabaseConfig().ConnMaxIdleTime).
		SetConnMaxLifetime(DefaultDatabaseConfig().ConnMaxLifetime).
		SetMaxIdleConns(DefaultDatabaseConfig().MaxIdleConns).
		SetMaxOpenConns(DefaultDatabaseConfig().MaxOpenConns))

	if err != nil {
		return fmt.Errorf("failed to register read replicas: %w", err)
	}

	logger.Info("Read replicas configured successfully",
		logx.Int("replicas", len(replicas)),
	)

	return nil
}

// sanitizeDSN removes sensitive information from DSN for logging
func sanitizeDSN(dsn string) string {
	// Simple sanitization - in production use proper URL parsing
	// This is a basic implementation
	return "[DSN redacted for security]"
}

// WithReadReplicas forces a query to use read replicas
// Usage: db.Clauses(dbresolver.Read).Find(&users)
func WithReadReplicas(db *gorm.DB) *gorm.DB {
	return db.Clauses(dbresolver.Read)
}

// WithPrimary forces a query to use the primary database
// Usage: db.Clauses(dbresolver.Write).Find(&users)
func WithPrimary(db *gorm.DB) *gorm.DB {
	return db.Clauses(dbresolver.Write)
}
