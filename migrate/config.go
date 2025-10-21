package migrate

import (
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents migration configuration
type Config struct {
	// AutoMigrate enables automatic migration on startup (default: false)
	// WARNING: Only use in development/CI environments, never in production
	AutoMigrate bool `mapstructure:"auto_migrate" yaml:"auto_migrate"`

	// Dir specifies the filesystem path to migration files
	// Example: "./dbx/migrate/files" or "/app/migrations"
	Dir string `mapstructure:"dir" yaml:"dir"`

	// UseEmbed indicates whether to use embedded migration files
	// When true, migrations are read from the embedded files/ directory
	UseEmbed bool `mapstructure:"use_embed" yaml:"use_embed"`

	// Table specifies the name of the schema migrations table
	// Default: "schema_migrations"
	Table string `mapstructure:"table" yaml:"table"`

	// LockTimeout specifies how long to wait for the migration lock
	// Default: 15 seconds
	LockTimeout time.Duration `mapstructure:"lock_timeout" yaml:"lock_timeout"`

	// Verbose enables verbose logging for migrations
	Verbose bool `mapstructure:"verbose" yaml:"verbose"`

	// EmbedFS holds the custom embedded filesystem (not serializable)
	EmbedFS embed.FS `mapstructure:"-" yaml:"-"`

	// EmbedSubdir specifies the subdirectory within the embedded filesystem
	EmbedSubdir string `mapstructure:"-" yaml:"-"`
}

// Option is a functional option for configuring migrations
type Option func(*Config)

// WithDir sets the filesystem directory for migrations
func WithDir(dir string) Option {
	return func(c *Config) {
		c.Dir = dir
		c.UseEmbed = false
	}
}

// WithEmbed enables embedded migration files
func WithEmbed() Option {
	return func(c *Config) {
		c.UseEmbed = true
		c.Dir = ""
	}
}

// WithTable sets a custom migration table name
func WithTable(name string) Option {
	return func(c *Config) {
		c.Table = name
	}
}

// WithLockTimeout sets the migration lock timeout
func WithLockTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.LockTimeout = d
	}
}

// WithVerbose enables verbose logging
func WithVerbose() Option {
	return func(c *Config) {
		c.Verbose = true
	}
}

// WithAutoMigrate enables auto-migration (use with caution!)
func WithAutoMigrate() Option {
	return func(c *Config) {
		c.AutoMigrate = true
	}
}

// DefaultConfig returns a Config with safe defaults
func DefaultConfig() *Config {
	return &Config{
		AutoMigrate: false, // Never default to true for safety
		Dir:         "",
		UseEmbed:    false,
		Table:       "schema_migrations",
		LockTimeout: 15 * time.Second,
		Verbose:     false,
	}
}

// Sanitize returns a shallow copy of the config with any sensitive values redacted.
// Migrations config doesn't typically have secrets, but we provide the method
// for consistency with other modules.
func (c *Config) Sanitize() *Config {
	if c == nil {
		return DefaultConfig()
	}
	copy := *c
	// no secrets expected here; ensure Dir isn't accidentally leaked in summaries
	return &copy
}

// ConfigSummary returns a small diagnostics map safe for logging.
func (c *Config) ConfigSummary() map[string]any {
	if c == nil {
		return map[string]any{}
	}
	return map[string]any{
		"auto_migrate": c.AutoMigrate,
		"use_embed":    c.UseEmbed,
		"has_dir":      c.Dir != "",
		"table":        c.Table,
	}
}

// NewConfig creates a new migration config from Viper using unified databases configuration
// Reads from databases.primary.* configuration keys and DB_DATABASES_PRIMARY_* environment variables
func NewConfig(v *viper.Viper) (*Config, error) {
	cfg := DefaultConfig()

	// Set defaults for unified config
	v.SetDefault("databases.primary.auto_migrate", false)
	v.SetDefault("databases.primary.migration_source", "")
	v.SetDefault("databases.primary.migration_table", "schema_migrations")
	v.SetDefault("databases.primary.migration_lock_timeout", "15s")
	v.SetDefault("databases.primary.migration_verbose", false)

	// Bind environment variables for unified config
	v.BindEnv("databases.primary.auto_migrate", "DB_DATABASES_PRIMARY_AUTO_MIGRATE")
	v.BindEnv("databases.primary.migration_source", "DB_DATABASES_PRIMARY_MIGRATION_SOURCE")
	v.BindEnv("databases.primary.migration_table", "DB_DATABASES_PRIMARY_MIGRATION_TABLE")
	v.BindEnv("databases.primary.migration_lock_timeout", "DB_DATABASES_PRIMARY_MIGRATION_LOCK_TIMEOUT")
	v.BindEnv("databases.primary.migration_verbose", "DB_DATABASES_PRIMARY_MIGRATION_VERBOSE")

	// Map unified config fields to migration config
	cfg.AutoMigrate = v.GetBool("databases.primary.auto_migrate")
	cfg.Table = v.GetString("databases.primary.migration_table")
	cfg.LockTimeout = v.GetDuration("databases.primary.migration_lock_timeout")
	cfg.Verbose = v.GetBool("databases.primary.migration_verbose")

	// Handle migration source
	source := v.GetString("databases.primary.migration_source")
	switch {
	case source == "embed://":
		cfg.UseEmbed = true
		cfg.Dir = ""
	case strings.HasPrefix(source, "file://"):
		cfg.UseEmbed = false
		cfg.Dir = source[7:] // Remove "file://" prefix
	case source != "":
		cfg.UseEmbed = false
		cfg.Dir = source // Use as-is (no prefix)
	}

	// Validate configuration
	// Sanitize before validation and returning
	cfg = cfg.Sanitize()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the migration configuration
func (c *Config) Validate() error {
	// Either Dir or UseEmbed must be set, but not both
	if c.Dir != "" && c.UseEmbed {
		return fmt.Errorf("%w: cannot specify both Dir and UseEmbed", ErrInvalidConfig)
	}

	// At least one source must be specified
	if c.Dir == "" && !c.UseEmbed {
		// This is OK for library usage where source is provided via options
		// but we'll warn if AutoMigrate is enabled
		if c.AutoMigrate {
			return fmt.Errorf("%w: AutoMigrate requires Dir or UseEmbed", ErrNoMigrationSource)
		}
	}

	// Validate table name
	if c.Table == "" {
		return fmt.Errorf("%w: table name cannot be empty", ErrInvalidConfig)
	}

	// Validate lock timeout
	if c.LockTimeout < 0 {
		return fmt.Errorf("%w: lock_timeout cannot be negative", ErrInvalidConfig)
	}

	return nil
}

// Apply applies functional options to the config
func (c *Config) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
}

// Clone creates a copy of the config
func (c *Config) Clone() *Config {
	if c == nil {
		return DefaultConfig()
	}
	return &Config{
		AutoMigrate: c.AutoMigrate,
		Dir:         c.Dir,
		UseEmbed:    c.UseEmbed,
		Table:       c.Table,
		LockTimeout: c.LockTimeout,
		Verbose:     c.Verbose,
	}
}
