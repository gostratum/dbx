package internal

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// NewRunnerWithEmbed creates a runner with an embedded filesystem source
func NewRunnerWithEmbed(ctx context.Context, dbURL string, embedFS embed.FS, subdir string, config *RunnerConfig) (*Runner, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	if config == nil {
		config = &RunnerConfig{
			Table:       "schema_migrations",
			LockTimeout: 15,
			Verbose:     false,
		}
	}

	// Create sub-filesystem
	var migrationsFS fs.FS
	var err error
	if subdir != "" {
		migrationsFS, err = fs.Sub(embedFS, subdir)
		if err != nil {
			return nil, fmt.Errorf("failed to create sub-filesystem: %w", err)
		}
	} else {
		migrationsFS = embedFS
	}

	// Create iofs source driver
	sourceDriver, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to create iofs source: %w", err)
	}

	// Open database connection
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Don't defer db.Close() because migrate will manage it

	// Create postgres driver instance
	dbDriver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: config.Table,
		DatabaseName:    "",
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Enable verbose logging if requested
	if config.Verbose {
		m.Log = &verboseLogger{}
	}

	return &Runner{
		migrate:   m,
		dbURL:     dbURL,
		sourceURL: "iofs://embed",
		config:    config,
	}, nil
}
