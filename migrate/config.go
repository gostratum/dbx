package migrate

import (
	"embed"
	"fmt"
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

// NewConfig creates a new migration config from Viper
// Supports both legacy "dbx.migrate" prefix and new unified "databases.primary" structure
// for backward compatibility
func NewConfig(v *viper.Viper) (*Config, error) {
	cfg := DefaultConfig()

	// Try new unified configuration first (databases.primary.*)
	if v.IsSet("databases.primary.migration_source") || v.IsSet("databases.primary.auto_migrate") {
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
		case source != "":
			cfg.UseEmbed = false
			cfg.Dir = source[7:] // Remove "file://" prefix if present
			if cfg.Dir == source {
				cfg.Dir = source // No prefix, use as-is
			}
		}
	} else {
		// Fall back to legacy dbx.migrate configuration
		v.SetDefault("dbx.migrate.auto_migrate", false)
		v.SetDefault("dbx.migrate.dir", "")
		v.SetDefault("dbx.migrate.use_embed", false)
		v.SetDefault("dbx.migrate.table", "schema_migrations")
		v.SetDefault("dbx.migrate.lock_timeout", "15s")
		v.SetDefault("dbx.migrate.verbose", false)

		// Bind environment variables for legacy config
		v.BindEnv("dbx.migrate.auto_migrate", "DBX_MIGRATE_AUTOMIGRATE", "DBX_MIGRATE_AUTO_MIGRATE")
		v.BindEnv("dbx.migrate.dir", "DBX_MIGRATE_DIR")
		v.BindEnv("dbx.migrate.use_embed", "DBX_MIGRATE_USE_EMBED", "DBX_MIGRATE_USEEMBED")
		v.BindEnv("dbx.migrate.table", "DBX_MIGRATE_TABLE")
		v.BindEnv("dbx.migrate.lock_timeout", "DBX_MIGRATE_LOCK_TIMEOUT", "DBX_MIGRATE_LOCKTIMEOUT")
		v.BindEnv("dbx.migrate.verbose", "DBX_MIGRATE_VERBOSE")

		// Unmarshal legacy configuration
		if err := v.UnmarshalKey("dbx.migrate", cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal migrate config: %w", err)
		}
	}

	// Validate configuration
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
