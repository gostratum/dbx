package migrate

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
)

var (
	// ErrNoChange indicates no migrations were applied
	ErrNoChange = migrate.ErrNoChange

	// ErrNilVersion indicates a nil version was encountered
	ErrNilVersion = migrate.ErrNilVersion

	// ErrLocked indicates the database is locked by another migration process
	ErrLocked = migrate.ErrLocked

	// ErrInvalidVersion indicates an invalid version number
	ErrInvalidVersion = errors.New("invalid migration version")

	// ErrNoMigrationSource indicates no migration source was configured
	ErrNoMigrationSource = errors.New("no migration source configured: specify Dir or UseEmbed")

	// ErrInvalidConfig indicates invalid migration configuration
	ErrInvalidConfig = errors.New("invalid migration configuration")

	// ErrDatabaseURLRequired indicates database URL is required
	ErrDatabaseURLRequired = errors.New("database URL is required")
)

// IsNoChange returns true if the error is ErrNoChange
func IsNoChange(err error) bool {
	return errors.Is(err, ErrNoChange)
}

// IsNilVersion returns true if the error is ErrNilVersion
func IsNilVersion(err error) bool {
	return errors.Is(err, ErrNilVersion)
}

// IsLocked returns true if the error is ErrLocked
func IsLocked(err error) bool {
	return errors.Is(err, ErrLocked)
}

// WrapError wraps migration errors with additional context
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}
