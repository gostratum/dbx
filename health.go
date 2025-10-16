package dbx

import (
	"context"
	"fmt"
	"time"

	"github.com/gostratum/core"
	"gorm.io/gorm"
)

// dbCheck implements core.Check for database health checks
type dbCheck struct {
	name      string
	kind      core.Kind
	checkFunc func(ctx context.Context) error
}

func (c *dbCheck) Name() string {
	return c.name
}

func (c *dbCheck) Kind() core.Kind {
	return c.kind
}

func (c *dbCheck) Check(ctx context.Context) error {
	return c.checkFunc(ctx)
}

// HealthChecker provides health check functionality for database connections
type HealthChecker struct {
	connections Connections
	registry    core.Registry
}

// NewHealthChecker creates a new health checker for database connections
func NewHealthChecker(connections Connections, registry core.Registry) *HealthChecker {
	return &HealthChecker{
		connections: connections,
		registry:    registry,
	}
}

// RegisterHealthChecks registers health checks for all database connections
func (hc *HealthChecker) RegisterHealthChecks() error {
	if hc.registry == nil {
		return nil // Skip if no registry provided
	}

	for name, db := range hc.connections {
		// Create readiness check
		readinessCheck := &dbCheck{
			name:      fmt.Sprintf("db-%s-readiness", name),
			kind:      core.Readiness,
			checkFunc: hc.createReadinessCheck(db),
		}

		// Create liveness check
		livenessCheck := &dbCheck{
			name:      fmt.Sprintf("db-%s-liveness", name),
			kind:      core.Liveness,
			checkFunc: hc.createLivenessCheck(db),
		}

		// Register checks
		hc.registry.Register(readinessCheck)
		hc.registry.Register(livenessCheck)
	}

	return nil
}

// createReadinessCheck creates a readiness check function for a database
func (hc *HealthChecker) createReadinessCheck(db *gorm.DB) func(ctx context.Context) error {
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
func (hc *HealthChecker) createLivenessCheck(db *gorm.DB) func(ctx context.Context) error {
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
