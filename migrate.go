package dbx

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MigrationRunner handles database migrations
type MigrationRunner struct {
	logger      *zap.Logger
	connections Connections
	fs          fs.FS
	dir         string
	autoMigrate []interface{}
	enabled     bool
}

// MigrationOption configures the migration runner
type MigrationOption func(*MigrationRunner)

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(logger *zap.Logger, connections Connections, opts ...MigrationOption) *MigrationRunner {
	runner := &MigrationRunner{
		logger:      logger.Named("migrations"),
		connections: connections,
		autoMigrate: make([]interface{}, 0),
		enabled:     false,
	}

	for _, opt := range opts {
		opt(runner)
	}

	return runner
}

// withMigrationsFS sets the embedded filesystem for SQL migrations
func withMigrationsFS(filesystem fs.FS, directory string) MigrationOption {
	return func(r *MigrationRunner) {
		r.fs = filesystem
		r.dir = directory
		r.enabled = true
	}
}

// withAutoMigrate sets models for auto-migration
func withAutoMigrate(models ...interface{}) MigrationOption {
	return func(r *MigrationRunner) {
		r.autoMigrate = append(r.autoMigrate, models...)
		r.enabled = true
	}
}

// RunMigrations executes all configured migrations
func (mr *MigrationRunner) RunMigrations() error {
	if !mr.enabled {
		mr.logger.Info("Migrations disabled, skipping")
		return nil
	}

	mr.logger.Info("Starting database migrations")

	// Run auto-migrations first
	if len(mr.autoMigrate) > 0 {
		if err := mr.runAutoMigrations(); err != nil {
			return fmt.Errorf("auto migration failed: %w", err)
		}
	}

	// Run SQL migrations
	if mr.fs != nil {
		if err := mr.runSQLMigrations(); err != nil {
			return fmt.Errorf("SQL migration failed: %w", err)
		}
	}

	mr.logger.Info("Database migrations completed successfully")
	return nil
}

// runAutoMigrations runs GORM auto-migrations
func (mr *MigrationRunner) runAutoMigrations() error {
	mr.logger.Info("Running auto-migrations", zap.Int("models", len(mr.autoMigrate)))

	for name, db := range mr.connections {
		mr.logger.Info("Running auto-migration for database", zap.String("database", name))

		if err := db.AutoMigrate(mr.autoMigrate...); err != nil {
			return fmt.Errorf("auto migration failed for database %s: %w", name, err)
		}

		mr.logger.Info("Auto-migration completed for database", zap.String("database", name))
	}

	return nil
}

// runSQLMigrations runs SQL file migrations
func (mr *MigrationRunner) runSQLMigrations() error {
	mr.logger.Info("Running SQL migrations", zap.String("directory", mr.dir))

	// Get migration files
	files, err := mr.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	if len(files) == 0 {
		mr.logger.Info("No SQL migration files found")
		return nil
	}

	// Create migration table if it doesn't exist
	if err := mr.createMigrationTable(); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// Run migrations for each database
	for name, db := range mr.connections {
		if err := mr.runMigrationsForDatabase(name, db, files); err != nil {
			return fmt.Errorf("migration failed for database %s: %w", name, err)
		}
	}

	return nil
}

// getMigrationFiles gets sorted list of migration files
func (mr *MigrationRunner) getMigrationFiles() ([]string, error) {
	var files []string

	err := fs.WalkDir(mr.fs, mr.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Only include .sql files
		if strings.HasSuffix(strings.ToLower(path), ".sql") {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort files to ensure consistent migration order
	sort.Strings(files)

	return files, nil
}

// createMigrationTable creates the migration tracking table
func (mr *MigrationRunner) createMigrationTable() error {
	createSQL := `
		CREATE TABLE IF NOT EXISTS dbx_migrations (
			id SERIAL PRIMARY KEY,
			filename VARCHAR(255) NOT NULL UNIQUE,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			checksum VARCHAR(64) NOT NULL
		)`

	for name, db := range mr.connections {
		mr.logger.Debug("Creating migration table", zap.String("database", name))

		if err := db.Exec(createSQL).Error; err != nil {
			return fmt.Errorf("failed to create migration table for database %s: %w", name, err)
		}
	}

	return nil
}

// runMigrationsForDatabase runs migrations for a specific database
func (mr *MigrationRunner) runMigrationsForDatabase(name string, db *gorm.DB, files []string) error {
	mr.logger.Info("Running migrations for database",
		zap.String("database", name),
		zap.Int("files", len(files)))

	for _, file := range files {
		if err := mr.runMigrationFile(name, db, file); err != nil {
			return fmt.Errorf("migration file %s failed: %w", file, err)
		}
	}

	mr.logger.Info("Migrations completed for database", zap.String("database", name))
	return nil
}

// runMigrationFile runs a single migration file
func (mr *MigrationRunner) runMigrationFile(dbName string, db *gorm.DB, filename string) error {
	// Check if migration was already executed
	var count int64
	if err := db.Table("dbx_migrations").
		Where("filename = ?", filepath.Base(filename)).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if count > 0 {
		mr.logger.Debug("Migration already executed, skipping",
			zap.String("database", dbName),
			zap.String("file", filename))
		return nil
	}

	// Read migration file
	content, err := fs.ReadFile(mr.fs, filename)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	mr.logger.Info("Executing migration",
		zap.String("database", dbName),
		zap.String("file", filename))

	// Execute migration in a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Execute the SQL
		if err := tx.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("failed to execute migration SQL: %w", err)
		}

		// Record migration
		migrationRecord := map[string]interface{}{
			"filename": filepath.Base(filename),
			"checksum": calculateChecksum(content),
		}

		if err := tx.Table("dbx_migrations").Create(migrationRecord).Error; err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		mr.logger.Info("Migration executed successfully",
			zap.String("database", dbName),
			zap.String("file", filename))

		return nil
	})
}

// calculateChecksum calculates a simple checksum for migration content
func calculateChecksum(content []byte) string {
	// Simple hash - in production, you might want to use SHA256
	hash := 0
	for _, b := range content {
		hash = hash*31 + int(b)
	}
	return fmt.Sprintf("%x", hash)
}
