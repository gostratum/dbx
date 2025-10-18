package internal

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Runner wraps golang-migrate operations
type Runner struct {
	migrate   *migrate.Migrate
	dbURL     string
	sourceURL string
	config    *RunnerConfig
}

// RunnerConfig holds runner configuration
type RunnerConfig struct {
	Table       string
	LockTimeout int // in seconds
	Verbose     bool
}

// NewRunner creates a new migration runner
func NewRunner(ctx context.Context, dbURL, sourceURL string, config *RunnerConfig) (*Runner, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	if sourceURL == "" {
		return nil, fmt.Errorf("source URL is required")
	}
	if config == nil {
		config = &RunnerConfig{
			Table:       "schema_migrations",
			LockTimeout: 15,
			Verbose:     false,
		}
	}

	// Create migrate instance using URLs
	// The library will handle both file:// and iofs:// sources when properly registered
	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Enable verbose logging if requested
	if config.Verbose {
		m.Log = &verboseLogger{}
	}

	return &Runner{
		migrate:   m,
		dbURL:     dbURL,
		sourceURL: sourceURL,
		config:    config,
	}, nil
}

// Up applies all pending migrations
func (r *Runner) Up(ctx context.Context) error {
	if err := r.migrate.Up(); err != nil {
		if err == migrate.ErrNoChange {
			return nil // Not an error, just no changes to apply
		}
		return fmt.Errorf("migration up failed: %w", err)
	}
	return nil
}

// Down reverts the most recent migration
func (r *Runner) Down(ctx context.Context) error {
	if err := r.migrate.Down(); err != nil {
		if err == migrate.ErrNoChange {
			return nil
		}
		return fmt.Errorf("migration down failed: %w", err)
	}
	return nil
}

// Steps applies n migrations (positive for up, negative for down)
func (r *Runner) Steps(ctx context.Context, n int) error {
	if err := r.migrate.Steps(n); err != nil {
		if err == migrate.ErrNoChange {
			return nil
		}
		return fmt.Errorf("migration steps failed: %w", err)
	}
	return nil
}

// To migrates to a specific version
func (r *Runner) To(ctx context.Context, version uint) error {
	if err := r.migrate.Migrate(version); err != nil {
		if err == migrate.ErrNoChange {
			return nil
		}
		return fmt.Errorf("migration to version %d failed: %w", version, err)
	}
	return nil
}

// Force sets the migration version without running migrations
// Use with extreme caution!
func (r *Runner) Force(ctx context.Context, version int) error {
	if err := r.migrate.Force(version); err != nil {
		return fmt.Errorf("force version %d failed: %w", version, err)
	}
	return nil
}

// Drop drops everything in the database
// WARNING: This is destructive and irreversible!
func (r *Runner) Drop(ctx context.Context) error {
	if err := r.migrate.Drop(); err != nil {
		return fmt.Errorf("drop failed: %w", err)
	}
	return nil
}

// Version returns the current migration version and dirty state
func (r *Runner) Version() (version uint, dirty bool, err error) {
	return r.migrate.Version()
}

// Close closes the migration runner
func (r *Runner) Close() error {
	srcErr, dbErr := r.migrate.Close()
	if srcErr != nil {
		return fmt.Errorf("failed to close source: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("failed to close database: %w", dbErr)
	}
	return nil
}

// verboseLogger implements migrate.Logger interface
type verboseLogger struct{}

func (l *verboseLogger) Printf(format string, v ...any) {
	fmt.Printf("[MIGRATE] "+format+"\n", v...)
}

func (l *verboseLogger) Verbose() bool {
	return true
}
