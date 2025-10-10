package dbx

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// HealthCheckFunc represents a health check function
type HealthCheckFunc func(ctx context.Context) error

// HealthCheck represents a health check configuration
type HealthCheck struct {
	Name        string
	Description string
	CheckFunc   HealthCheckFunc
	Timeout     time.Duration
	Tags        []string
}

// HealthRegistry interface for registering health checks
type HealthRegistry interface {
	RegisterReadinessCheck(check *HealthCheck) error
	RegisterLivenessCheck(check *HealthCheck) error
}

// HealthChecker provides health check functionality for database connections
type HealthChecker struct {
	connections Connections
	registry    HealthRegistry
}

// NewHealthChecker creates a new health checker for database connections
func NewHealthChecker(connections Connections, registry HealthRegistry) *HealthChecker {
	return &HealthChecker{
		connections: connections,
		registry:    registry,
	}
}

// RegisterHealthChecks registers health checks for all database connections
func (hc *HealthChecker) RegisterHealthChecks() error {
	for name, db := range hc.connections {
		// Create readiness check
		readinessCheck := &HealthCheck{
			Name:        fmt.Sprintf("db-%s-readiness", name),
			Description: fmt.Sprintf("Database %s readiness check", name),
			CheckFunc:   hc.createReadinessCheck(db),
			Timeout:     5 * time.Second,
			Tags:        []string{"database", "readiness", name},
		}

		// Create liveness check
		livenessCheck := &HealthCheck{
			Name:        fmt.Sprintf("db-%s-liveness", name),
			Description: fmt.Sprintf("Database %s liveness check", name),
			CheckFunc:   hc.createLivenessCheck(db),
			Timeout:     10 * time.Second,
			Tags:        []string{"database", "liveness", name},
		}

		// Register checks
		if err := hc.registry.RegisterReadinessCheck(readinessCheck); err != nil {
			return fmt.Errorf("failed to register readiness check for db %s: %w", name, err)
		}

		if err := hc.registry.RegisterLivenessCheck(livenessCheck); err != nil {
			return fmt.Errorf("failed to register liveness check for db %s: %w", name, err)
		}
	}

	return nil
}

// createReadinessCheck creates a readiness check function for a database
func (hc *HealthChecker) createReadinessCheck(db *gorm.DB) HealthCheckFunc {
	return func(ctx context.Context) error {
		// Get the underlying sql.DB
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("failed to get underlying DB: %w", err)
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Ping the database
		if err := sqlDB.PingContext(ctx); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}

		return nil
	}
}

// createLivenessCheck creates a liveness check function for a database
func (hc *HealthChecker) createLivenessCheck(db *gorm.DB) HealthCheckFunc {
	return func(ctx context.Context) error {
		// Get the underlying sql.DB
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("failed to get underlying DB: %w", err)
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Ping the database
		if err := sqlDB.PingContext(ctx); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}

		// Check connection pool stats
		stats := sqlDB.Stats()
		if stats.OpenConnections > stats.MaxOpenConnections && stats.MaxOpenConnections > 0 {
			return fmt.Errorf("connection pool exhausted: %d/%d connections in use",
				stats.OpenConnections, stats.MaxOpenConnections)
		}

		// Execute a simple query to ensure the database is responsive
		var result int
		if err := db.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error; err != nil {
			return fmt.Errorf("database query failed: %w", err)
		}

		if result != 1 {
			return fmt.Errorf("database query returned unexpected result: %d", result)
		}

		return nil
	}
}

// GetConnectionStats returns connection statistics for all databases
func (hc *HealthChecker) GetConnectionStats() (map[string]ConnectionStats, error) {
	stats := make(map[string]ConnectionStats)

	for name, db := range hc.connections {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get underlying DB for %s: %w", name, err)
		}

		dbStats := sqlDB.Stats()
		stats[name] = ConnectionStats{
			MaxOpenConnections: dbStats.MaxOpenConnections,
			OpenConnections:    dbStats.OpenConnections,
			InUse:              dbStats.InUse,
			Idle:               dbStats.Idle,
			WaitCount:          dbStats.WaitCount,
			WaitDuration:       dbStats.WaitDuration,
			MaxIdleClosed:      dbStats.MaxIdleClosed,
			MaxLifetimeClosed:  dbStats.MaxLifetimeClosed,
		}
	}

	return stats, nil
}

// ConnectionStats represents database connection pool statistics
type ConnectionStats struct {
	MaxOpenConnections int           `json:"max_open_connections"`
	OpenConnections    int           `json:"open_connections"`
	InUse              int           `json:"in_use"`
	Idle               int           `json:"idle"`
	WaitCount          int64         `json:"wait_count"`
	WaitDuration       time.Duration `json:"wait_duration"`
	MaxIdleClosed      int64         `json:"max_idle_closed"`
	MaxLifetimeClosed  int64         `json:"max_lifetime_closed"`
}
