package migrate

import (
	"context"
	"time"

	"github.com/gostratum/metricsx"
)

// MigrationMetrics provides metrics tracking for database migrations
type MigrationMetrics struct {
	metrics metricsx.Metrics

	// Metric collectors
	operationsTotal   metricsx.Counter
	operationDuration metricsx.Histogram
	currentVersion    metricsx.Gauge
	pendingCount      metricsx.Gauge
	dirtyState        metricsx.Gauge
}

// NewMigrationMetrics creates a new migration metrics tracker
func NewMigrationMetrics(metrics metricsx.Metrics) *MigrationMetrics {
	if metrics == nil {
		return nil
	}

	m := &MigrationMetrics{
		metrics: metrics,
	}

	// Initialize metric collectors
	m.operationsTotal = metrics.Counter(
		"db_migration_operations_total",
		metricsx.WithHelp("Total number of migration operations"),
		metricsx.WithLabels("database", "operation", "status"),
	)

	m.operationDuration = metrics.Histogram(
		"db_migration_duration_seconds",
		metricsx.WithHelp("Migration operation duration in seconds"),
		metricsx.WithLabels("database", "operation"),
		metricsx.WithBuckets(0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0),
	)

	m.currentVersion = metrics.Gauge(
		"db_migration_version_current",
		metricsx.WithHelp("Current migration version"),
		metricsx.WithLabels("database"),
	)

	m.pendingCount = metrics.Gauge(
		"db_migration_pending_count",
		metricsx.WithHelp("Number of pending migrations"),
		metricsx.WithLabels("database"),
	)

	m.dirtyState = metrics.Gauge(
		"db_migration_dirty",
		metricsx.WithHelp("Whether migration is in dirty state (0=clean, 1=dirty)"),
		metricsx.WithLabels("database"),
	)

	return m
}

// RecordOperation records a migration operation with its duration and outcome
func (m *MigrationMetrics) RecordOperation(database, operation string, duration time.Duration, err error) {
	if m == nil {
		return
	}

	status := "success"
	if err != nil {
		status = "failure"
	}

	m.operationsTotal.Inc(database, operation, status)
	m.operationDuration.Observe(duration.Seconds(), database, operation)
}

// UpdateStatus updates the migration status metrics
func (m *MigrationMetrics) UpdateStatus(database string, version uint, dirty bool, pendingCount int) {
	if m == nil {
		return
	}

	m.currentVersion.Set(float64(version), database)
	m.pendingCount.Set(float64(pendingCount), database)

	dirtyValue := 0.0
	if dirty {
		dirtyValue = 1.0
	}
	m.dirtyState.Set(dirtyValue, database)
}

// trackOperation is a helper to track operation duration and result
func (m *MigrationMetrics) trackOperation(database, operation string, fn func() error) error {
	if m == nil {
		return fn()
	}

	start := time.Now()
	err := fn()
	duration := time.Since(start)

	m.RecordOperation(database, operation, duration, err)

	return err
}

// WithMetrics wraps a migration operation to track metrics
func WithMetrics(ctx context.Context, dbURL string, metrics *MigrationMetrics, operation string, fn func(context.Context, string) error) error {
	if metrics == nil {
		return fn(ctx, dbURL)
	}

	// Extract database name from URL (simple extraction for metrics label)
	database := extractDatabaseFromURL(dbURL)

	return metrics.trackOperation(database, operation, func() error {
		return fn(ctx, dbURL)
	})
}

// extractDatabaseFromURL extracts a simple database identifier from connection URL
// This is used for metrics labels only, not for actual connection
func extractDatabaseFromURL(dbURL string) string {
	// Simple heuristic: use "default" as database name
	// In real usage, the caller can provide a better database name via context
	// or configuration
	return "default"
}

// UpWithMetrics is like Up but tracks metrics
func UpWithMetrics(ctx context.Context, dbURL string, metrics *MigrationMetrics, opts ...Option) error {
	if metrics == nil {
		return Up(ctx, dbURL, opts...)
	}

	database := extractDatabaseFromURL(dbURL)
	err := metrics.trackOperation(database, "up", func() error {
		return Up(ctx, dbURL, opts...)
	})

	// Update status after operation
	if status, statusErr := GetStatus(ctx, dbURL, opts...); statusErr == nil {
		metrics.UpdateStatus(database, status.Current, status.Dirty, len(status.Pending))
	}

	return err
}

// DownWithMetrics is like Down but tracks metrics
func DownWithMetrics(ctx context.Context, dbURL string, metrics *MigrationMetrics, opts ...Option) error {
	if metrics == nil {
		return Down(ctx, dbURL, opts...)
	}

	database := extractDatabaseFromURL(dbURL)
	err := metrics.trackOperation(database, "down", func() error {
		return Down(ctx, dbURL, opts...)
	})

	// Update status after operation
	if status, statusErr := GetStatus(ctx, dbURL, opts...); statusErr == nil {
		metrics.UpdateStatus(database, status.Current, status.Dirty, len(status.Pending))
	}

	return err
}

// StepsWithMetrics is like Steps but tracks metrics
func StepsWithMetrics(ctx context.Context, dbURL string, n int, metrics *MigrationMetrics, opts ...Option) error {
	if metrics == nil {
		return Steps(ctx, dbURL, n, opts...)
	}

	database := extractDatabaseFromURL(dbURL)
	operation := "steps_up"
	if n < 0 {
		operation = "steps_down"
	}

	err := metrics.trackOperation(database, operation, func() error {
		return Steps(ctx, dbURL, n, opts...)
	})

	// Update status after operation
	if status, statusErr := GetStatus(ctx, dbURL, opts...); statusErr == nil {
		metrics.UpdateStatus(database, status.Current, status.Dirty, len(status.Pending))
	}

	return err
}

// ToWithMetrics is like To but tracks metrics
func ToWithMetrics(ctx context.Context, dbURL string, version uint, metrics *MigrationMetrics, opts ...Option) error {
	if metrics == nil {
		return To(ctx, dbURL, version, opts...)
	}

	database := extractDatabaseFromURL(dbURL)
	err := metrics.trackOperation(database, "to", func() error {
		return To(ctx, dbURL, version, opts...)
	})

	// Update status after operation
	if status, statusErr := GetStatus(ctx, dbURL, opts...); statusErr == nil {
		metrics.UpdateStatus(database, status.Current, status.Dirty, len(status.Pending))
	}

	return err
}

// ForceWithMetrics is like Force but tracks metrics
func ForceWithMetrics(ctx context.Context, dbURL string, version int, metrics *MigrationMetrics, opts ...Option) error {
	if metrics == nil {
		return Force(ctx, dbURL, version, opts...)
	}

	database := extractDatabaseFromURL(dbURL)
	err := metrics.trackOperation(database, "force", func() error {
		return Force(ctx, dbURL, version, opts...)
	})

	// Update status after operation
	if status, statusErr := GetStatus(ctx, dbURL, opts...); statusErr == nil {
		metrics.UpdateStatus(database, status.Current, status.Dirty, len(status.Pending))
	}

	return err
}

// DropWithMetrics is like Drop but tracks metrics
func DropWithMetrics(ctx context.Context, dbURL string, metrics *MigrationMetrics, opts ...Option) error {
	if metrics == nil {
		return Drop(ctx, dbURL, opts...)
	}

	database := extractDatabaseFromURL(dbURL)
	return metrics.trackOperation(database, "drop", func() error {
		return Drop(ctx, dbURL, opts...)
	})
}
