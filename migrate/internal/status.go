package internal

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// MigrationInfo represents information about a single migration
type MigrationInfo struct {
	Version uint
	Name    string
	Applied bool
}

// Status represents the current migration status
type Status struct {
	DatabaseURL string
	Current     uint
	Dirty       bool
	Applied     []uint
	Pending     []uint
}

// GetStatus retrieves the current migration status
func GetStatus(ctx context.Context, dbURL, sourceURL, tableName string) (*Status, error) {
	// Create a new runner to get status
	runner, err := NewRunner(ctx, dbURL, sourceURL, &RunnerConfig{
		Table:   tableName,
		Verbose: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner for status: %w", err)
	}
	defer runner.Close()

	// Get current version and dirty state
	version, dirty, err := runner.Version()
	if err != nil {
		// If no migrations have been run yet, version returns error
		// We treat this as version 0
		version = 0
		dirty = false
	}

	status := &Status{
		DatabaseURL: dbURL,
		Current:     version,
		Dirty:       dirty,
		Applied:     []uint{},
		Pending:     []uint{},
	}

	// Get applied migrations from the database
	applied, err := getAppliedMigrations(ctx, dbURL, tableName)
	if err != nil {
		// If table doesn't exist yet, no migrations have been applied
		status.Applied = []uint{}
	} else {
		status.Applied = applied
	}

	return status, nil
}

// getAppliedMigrations queries the database for applied migrations
func getAppliedMigrations(ctx context.Context, dbURL, tableName string) ([]uint, error) {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Check if migrations table exists
	var exists bool
	query := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_name = $1
	)`
	if err := db.QueryRowContext(ctx, query, tableName).Scan(&exists); err != nil {
		return nil, fmt.Errorf("failed to check migrations table: %w", err)
	}

	if !exists {
		return []uint{}, nil
	}

	// Query applied migrations
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT version FROM %s WHERE dirty = false ORDER BY version", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var applied []uint
	for rows.Next() {
		var version uint
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied = append(applied, version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migration rows: %w", err)
	}

	return applied, nil
}
