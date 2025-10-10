package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/gostratum/dbx/migrate/internal"
)

// Status represents the current migration status
type Status struct {
	DatabaseURL string
	Current     uint
	Dirty       bool
	Applied     []uint
	Pending     []uint
}

// Up applies all pending migrations
func Up(ctx context.Context, dbURL string, opts ...Option) error {
	cfg := DefaultConfig()
	cfg.Apply(opts...)

	if err := cfg.Validate(); err != nil {
		return err
	}

	runner, err := createRunner(ctx, dbURL, cfg)
	if err != nil {
		return WrapError(err, "failed to create migration runner")
	}
	defer runner.Close()

	if err := runner.Up(ctx); err != nil {
		return WrapError(err, "migration up failed")
	}

	return nil
}

// Down reverts the most recent migration
func Down(ctx context.Context, dbURL string, opts ...Option) error {
	cfg := DefaultConfig()
	cfg.Apply(opts...)

	if err := cfg.Validate(); err != nil {
		return err
	}

	runner, err := createRunner(ctx, dbURL, cfg)
	if err != nil {
		return WrapError(err, "failed to create migration runner")
	}
	defer runner.Close()

	if err := runner.Down(ctx); err != nil {
		return WrapError(err, "migration down failed")
	}

	return nil
}

// Steps applies n migrations (positive for up, negative for down)
func Steps(ctx context.Context, dbURL string, n int, opts ...Option) error {
	cfg := DefaultConfig()
	cfg.Apply(opts...)

	if err := cfg.Validate(); err != nil {
		return err
	}

	runner, err := createRunner(ctx, dbURL, cfg)
	if err != nil {
		return WrapError(err, "failed to create migration runner")
	}
	defer runner.Close()

	if err := runner.Steps(ctx, n); err != nil {
		return WrapError(err, fmt.Sprintf("migration steps (%d) failed", n))
	}

	return nil
}

// To migrates to a specific version
func To(ctx context.Context, dbURL string, version uint, opts ...Option) error {
	cfg := DefaultConfig()
	cfg.Apply(opts...)

	if err := cfg.Validate(); err != nil {
		return err
	}

	runner, err := createRunner(ctx, dbURL, cfg)
	if err != nil {
		return WrapError(err, "failed to create migration runner")
	}
	defer runner.Close()

	if err := runner.To(ctx, version); err != nil {
		return WrapError(err, fmt.Sprintf("migration to version %d failed", version))
	}

	return nil
}

// Force sets the migration version without running migrations
// WARNING: Use with extreme caution! This can corrupt your migration state.
func Force(ctx context.Context, dbURL string, version int, opts ...Option) error {
	cfg := DefaultConfig()
	cfg.Apply(opts...)

	if err := cfg.Validate(); err != nil {
		return err
	}

	runner, err := createRunner(ctx, dbURL, cfg)
	if err != nil {
		return WrapError(err, "failed to create migration runner")
	}
	defer runner.Close()

	if err := runner.Force(ctx, version); err != nil {
		return WrapError(err, fmt.Sprintf("force version %d failed", version))
	}

	return nil
}

// Drop drops everything in the database
// WARNING: This is destructive and irreversible!
func Drop(ctx context.Context, dbURL string, opts ...Option) error {
	cfg := DefaultConfig()
	cfg.Apply(opts...)

	if err := cfg.Validate(); err != nil {
		return err
	}

	runner, err := createRunner(ctx, dbURL, cfg)
	if err != nil {
		return WrapError(err, "failed to create migration runner")
	}
	defer runner.Close()

	if err := runner.Drop(ctx); err != nil {
		return WrapError(err, "drop failed")
	}

	return nil
}

// GetStatus retrieves the current migration status
func GetStatus(ctx context.Context, dbURL string, opts ...Option) (Status, error) {
	cfg := DefaultConfig()
	cfg.Apply(opts...)

	// For status, we don't strictly need source validation
	if dbURL == "" {
		return Status{}, ErrDatabaseURLRequired
	}

	// We still need source URL for getting pending migrations list
	// But we can be more lenient here
	if cfg.Dir == "" && !cfg.UseEmbed {
		// Just return current version from DB without source info
		return getStatusWithoutSource(ctx, dbURL, cfg.Table)
	}

	runner, err := createRunner(ctx, dbURL, cfg)
	if err != nil {
		return Status{}, WrapError(err, "failed to create migration runner")
	}
	defer runner.Close()

	version, dirty, err := runner.Version()
	if err != nil {
		// No migrations applied yet
		version = 0
		dirty = false
	}

	status, err := internal.GetStatus(ctx, dbURL, "", cfg.Table)
	if err != nil {
		return Status{}, WrapError(err, "failed to get migration status")
	}

	return Status{
		DatabaseURL: dbURL,
		Current:     version,
		Dirty:       dirty,
		Applied:     status.Applied,
		Pending:     status.Pending,
	}, nil
}

// getStatusWithoutSource gets status when no source is available
func getStatusWithoutSource(ctx context.Context, dbURL, tableName string) (Status, error) {
	status, err := internal.GetStatus(ctx, dbURL, "", tableName)
	if err != nil {
		return Status{}, WrapError(err, "failed to get migration status")
	}

	return Status{
		DatabaseURL: status.DatabaseURL,
		Current:     status.Current,
		Dirty:       status.Dirty,
		Applied:     status.Applied,
		Pending:     status.Pending,
	}, nil
}

// createRunner creates a migration runner based on config
func createRunner(ctx context.Context, dbURL string, cfg *Config) (*internal.Runner, error) {
	runnerCfg := &internal.RunnerConfig{
		Table:       cfg.Table,
		LockTimeout: int(cfg.LockTimeout.Seconds()),
		Verbose:     cfg.Verbose,
	}

	if cfg.UseEmbed {
		// Use embedded migrations - prefer custom EmbedFS if provided
		embedFS := cfg.EmbedFS
		subdir := cfg.EmbedSubdir

		// Fallback to default embedded migrations if custom FS is not provided
		if dirs, err := embedFS.ReadDir("."); err != nil || len(dirs) == 0 {
			embedFS = EmbeddedMigrations
			subdir = "files"
		}

		return internal.NewRunnerWithEmbed(ctx, dbURL, embedFS, subdir, runnerCfg)
	}

	if cfg.Dir != "" {
		// Use filesystem migrations
		sourceURL, err := newFileSourceURL(cfg.Dir)
		if err != nil {
			return nil, err
		}
		return internal.NewRunner(ctx, dbURL, sourceURL, runnerCfg)
	}

	return nil, ErrNoMigrationSource
}

// UpFromDatabaseConfig applies all pending migrations using database configuration
// This provides a more integrated approach using the DatabaseConfig directly
func UpFromDatabaseConfig(ctx context.Context, dbConfig DatabaseConfigInterface) error {
	// Check if migrations are enabled
	if dbConfig.GetMigrationSource() == "" {
		return ErrNoMigrationSource
	}

	// Build options from database config
	var opts []Option

	if dbConfig.GetMigrationSource() == "embed://" {
		opts = append(opts, WithEmbed())
	} else if len(dbConfig.GetMigrationSource()) > 7 && dbConfig.GetMigrationSource()[:7] == "file://" {
		dir := dbConfig.GetMigrationSource()[7:] // Remove "file://" prefix
		opts = append(opts, WithDir(dir))
	} else {
		return fmt.Errorf("invalid migration_source format: %s", dbConfig.GetMigrationSource())
	}

	opts = append(opts,
		WithTable(dbConfig.GetMigrationTable()),
		WithLockTimeout(dbConfig.GetMigrationLockTimeout()),
	)

	if dbConfig.GetMigrationVerbose() {
		opts = append(opts, WithVerbose())
	}

	return Up(ctx, dbConfig.GetDSN(), opts...)
}

// DatabaseConfigInterface defines the interface for database configuration
// This allows the migrate package to work with database configs without circular imports
type DatabaseConfigInterface interface {
	GetDSN() string
	GetMigrationSource() string
	GetMigrationTable() string
	GetMigrationLockTimeout() time.Duration
	GetMigrationVerbose() bool
}
