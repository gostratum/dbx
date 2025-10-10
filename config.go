package dbx

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the database configuration
type Config struct {
	Databases map[string]*DatabaseConfig `mapstructure:"databases" yaml:"databases"`
	Default   string                     `mapstructure:"default" yaml:"default"`
}

// DatabaseConfig represents configuration for a single database connection
type DatabaseConfig struct {
	// Database Connection Settings
	Driver          string            `mapstructure:"driver" yaml:"driver"`
	DSN             string            `mapstructure:"dsn" yaml:"dsn"`
	MaxOpenConns    int               `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int               `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration     `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration     `mapstructure:"conn_max_idle_time" yaml:"conn_max_idle_time"`
	LogLevel        string            `mapstructure:"log_level" yaml:"log_level"`
	SlowThreshold   time.Duration     `mapstructure:"slow_threshold" yaml:"slow_threshold"`
	SkipDefaultTx   bool              `mapstructure:"skip_default_tx" yaml:"skip_default_tx"`
	PrepareStmt     bool              `mapstructure:"prepare_stmt" yaml:"prepare_stmt"`
	Params          map[string]string `mapstructure:"params" yaml:"params"`

	// Migration Settings
	// MigrationSource defines where migration files are located
	// Formats:
	//   - "file://./migrations" - Read from filesystem directory
	//   - "embed://" - Use embedded files (requires //go:embed in your app)
	//   - "" (empty) - No migrations (migrations disabled)
	MigrationSource string `mapstructure:"migration_source" yaml:"migration_source"`

	// AutoMigrate enables automatic migration on startup (default: false)
	// WARNING: Only use in development/CI environments, NEVER in production
	AutoMigrate bool `mapstructure:"auto_migrate" yaml:"auto_migrate"`

	// MigrationTable specifies the name of the schema migrations table
	// Default: "schema_migrations"
	MigrationTable string `mapstructure:"migration_table" yaml:"migration_table"`

	// MigrationLockTimeout specifies how long to wait for the migration lock
	// Default: 15 seconds
	MigrationLockTimeout time.Duration `mapstructure:"migration_lock_timeout" yaml:"migration_lock_timeout"`

	// MigrationVerbose enables verbose logging for migrations
	MigrationVerbose bool `mapstructure:"migration_verbose" yaml:"migration_verbose"`
}

// DefaultConfig returns the default database configuration
func DefaultConfig() *Config {
	return &Config{
		Databases: map[string]*DatabaseConfig{
			"primary": DefaultDatabaseConfig(),
		},
		Default: "primary",
	}
}

// DefaultDatabaseConfig returns the default configuration for a database connection
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		// Database Connection Settings
		Driver:          "postgres",
		DSN:             "postgres://localhost/dbname?sslmode=disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		LogLevel:        "warn",
		SlowThreshold:   200 * time.Millisecond,
		SkipDefaultTx:   false,
		PrepareStmt:     true,
		Params:          make(map[string]string),

		// Migration Settings (Safe Defaults)
		MigrationSource:      "",                  // Disabled by default for safety
		AutoMigrate:          false,               // NEVER enable by default (production safety)
		MigrationTable:       "schema_migrations", // Standard table name
		MigrationLockTimeout: 15 * time.Second,    // Reasonable lock timeout
		MigrationVerbose:     false,               // Quiet by default
	}
}

// LoadConfig loads database configuration from viper
func LoadConfig(v *viper.Viper) (*Config, error) {
	// Set defaults
	setDefaults(v)

	// Unmarshal into config struct
	cfg := &Config{}
	if err := v.UnmarshalKey("db", cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal db config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid db config: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Databases) == 0 {
		return fmt.Errorf("no databases configured")
	}

	// Check if default database exists
	if c.Default != "" {
		if _, exists := c.Databases[c.Default]; !exists {
			return fmt.Errorf("default database '%s' not found in configured databases", c.Default)
		}
	}

	// Validate each database config
	for name, dbConfig := range c.Databases {
		if err := dbConfig.Validate(); err != nil {
			return fmt.Errorf("database '%s': %w", name, err)
		}
	}

	return nil
}

// Validate validates a database configuration
func (dc *DatabaseConfig) Validate() error {
	if dc.Driver == "" {
		return fmt.Errorf("driver is required")
	}

	if dc.DSN == "" {
		return fmt.Errorf("dsn is required")
	}

	if dc.MaxOpenConns < 0 {
		return fmt.Errorf("max_open_conns must be >= 0")
	}

	if dc.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns must be >= 0")
	}

	if dc.ConnMaxLifetime < 0 {
		return fmt.Errorf("conn_max_lifetime must be >= 0")
	}

	if dc.ConnMaxIdleTime < 0 {
		return fmt.Errorf("conn_max_idle_time must be >= 0")
	}

	// Validate migration settings
	if dc.AutoMigrate && dc.MigrationSource == "" {
		return fmt.Errorf("auto_migrate is enabled but migration_source is empty - specify 'file://./migrations' or 'embed://'")
	}

	if dc.MigrationSource != "" {
		if dc.MigrationSource != "embed://" && !isValidFileURL(dc.MigrationSource) {
			return fmt.Errorf("migration_source must be 'embed://' or 'file://path' format, got: %s", dc.MigrationSource)
		}
	}

	if dc.MigrationTable == "" && dc.MigrationSource != "" {
		return fmt.Errorf("migration_table cannot be empty when migration_source is specified")
	}

	if dc.MigrationLockTimeout < 0 {
		return fmt.Errorf("migration_lock_timeout must be >= 0")
	}

	return nil
}

// isValidFileURL checks if a string is a valid file:// URL
func isValidFileURL(s string) bool {
	return len(s) > 7 && s[:7] == "file://"
}

// Migration interface methods for DatabaseConfig
// These allow the migrate package to use DatabaseConfig without circular imports

// GetDSN returns the database DSN
func (dc *DatabaseConfig) GetDSN() string {
	return dc.DSN
}

// GetMigrationSource returns the migration source
func (dc *DatabaseConfig) GetMigrationSource() string {
	return dc.MigrationSource
}

// GetMigrationTable returns the migration table name
func (dc *DatabaseConfig) GetMigrationTable() string {
	return dc.MigrationTable
}

// GetMigrationLockTimeout returns the migration lock timeout
func (dc *DatabaseConfig) GetMigrationLockTimeout() time.Duration {
	return dc.MigrationLockTimeout
}

// GetMigrationVerbose returns whether migration verbose logging is enabled
func (dc *DatabaseConfig) GetMigrationVerbose() bool {
	return dc.MigrationVerbose
}

// GetDefaultDatabase returns the default database configuration
func (c *Config) GetDefaultDatabase() (*DatabaseConfig, error) {
	if c.Default == "" {
		// Return the first database if no default is set
		for _, dbConfig := range c.Databases {
			return dbConfig, nil
		}
		return nil, fmt.Errorf("no databases configured")
	}

	dbConfig, exists := c.Databases[c.Default]
	if !exists {
		return nil, fmt.Errorf("default database '%s' not found", c.Default)
	}

	return dbConfig, nil
}

// setDefaults sets default values for database configuration
func setDefaults(v *viper.Viper) {
	// Set default database configuration
	v.SetDefault("db.default", "primary")
	v.SetDefault("db.databases.primary.driver", "postgres")
	v.SetDefault("db.databases.primary.dsn", "postgres://localhost/dbname?sslmode=disable")
	v.SetDefault("db.databases.primary.max_open_conns", 25)
	v.SetDefault("db.databases.primary.max_idle_conns", 5)
	v.SetDefault("db.databases.primary.conn_max_lifetime", "5m")
	v.SetDefault("db.databases.primary.conn_max_idle_time", "5m")
	v.SetDefault("db.databases.primary.log_level", "warn")
	v.SetDefault("db.databases.primary.slow_threshold", "200ms")
	v.SetDefault("db.databases.primary.skip_default_tx", false)
	v.SetDefault("db.databases.primary.prepare_stmt", true)

	// Set default migration configuration (safe defaults)
	v.SetDefault("db.databases.primary.migration_source", "") // Disabled by default
	v.SetDefault("db.databases.primary.auto_migrate", false)  // NEVER enable by default
	v.SetDefault("db.databases.primary.migration_table", "schema_migrations")
	v.SetDefault("db.databases.primary.migration_lock_timeout", "15s")
	v.SetDefault("db.databases.primary.migration_verbose", false)
}
