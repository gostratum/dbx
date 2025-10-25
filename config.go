package dbx

import (
	"fmt"
	"time"
)

// Config represents the database configuration
type Config struct {
	Databases map[string]*DatabaseConfig `mapstructure:"databases" yaml:"databases" default:"{}"`
	Default   string                     `mapstructure:"default" yaml:"default" default:"primary"`
}

// DatabaseConfig represents configuration for a single database connection
type DatabaseConfig struct {
	// Database Connection Settings
	Driver string `mapstructure:"driver" yaml:"driver" default:"postgres"`
	DSN    string `mapstructure:"dsn" yaml:"dsn"`

	// Read Replicas - for read/write splitting
	ReadReplicas []string `mapstructure:"read_replicas" yaml:"read_replicas"`

	// Connection components (optional) - if DSN is not provided these are used to build one
	Host            string            `mapstructure:"host" yaml:"host"`
	Port            int               `mapstructure:"port" yaml:"port"`
	User            string            `mapstructure:"user" yaml:"user"`
	Password        string            `mapstructure:"password" yaml:"password"`
	DBName          string            `mapstructure:"dbname" yaml:"dbname"`
	SSLMode         string            `mapstructure:"sslmode" yaml:"sslmode"`
	MaxOpenConns    int               `mapstructure:"max_open_conns" yaml:"max_open_conns" default:"25"`
	MaxIdleConns    int               `mapstructure:"max_idle_conns" yaml:"max_idle_conns" default:"5"`
	ConnMaxLifetime time.Duration     `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime" default:"5m"`
	ConnMaxIdleTime time.Duration     `mapstructure:"conn_max_idle_time" yaml:"conn_max_idle_time" default:"5m"`
	LogLevel        string            `mapstructure:"log_level" yaml:"log_level" default:"warn"`
	SlowThreshold   time.Duration     `mapstructure:"slow_threshold" yaml:"slow_threshold" default:"200ms"`
	SkipDefaultTx   bool              `mapstructure:"skip_default_tx" yaml:"skip_default_tx" default:"false"`
	PrepareStmt     bool              `mapstructure:"prepare_stmt" yaml:"prepare_stmt" default:"true"`
	Params          map[string]string `mapstructure:"params" yaml:"params"`

	// Migration Settings
	// MigrationSource defines where migration files are located
	// Formats:
	//   - "file://./migrations" - Read from filesystem directory
	//   - "embed://" - Use embedded files (requires //go:embed in your app)
	//   - "" (empty) - No migrations (migrations disabled)
	MigrationSource string `mapstructure:"migration_source" yaml:"migration_source" default:""`

	// AutoMigrate enables automatic migration on startup (default: false)
	// WARNING: Only use in development/CI environments, NEVER in production
	AutoMigrate bool `mapstructure:"auto_migrate" yaml:"auto_migrate" default:"false"`

	// MigrationTable specifies the name of the schema migrations table
	// Default: "schema_migrations"
	MigrationTable string `mapstructure:"migration_table" yaml:"migration_table" default:"schema_migrations"`

	// MigrationLockTimeout specifies how long to wait for the migration lock
	// Default: 15 seconds
	MigrationLockTimeout time.Duration `mapstructure:"migration_lock_timeout" yaml:"migration_lock_timeout" default:"15s"`

	// MigrationVerbose enables verbose logging for migrations
	MigrationVerbose bool `mapstructure:"migration_verbose" yaml:"migration_verbose" default:"false"`
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
		Host:            "localhost",
		Port:            5432,
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

// Prefix returns the configuration prefix for this module
// This implements the configx.Configurable interface
func (c *Config) Prefix() string {
	return "db"
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

	// Accept either a full DSN or component fields (DBName at minimum)
	if dc.DSN == "" {
		if dc.DBName == "" {
			return fmt.Errorf("dsn is required or provide dbname/user/host to build one")
		}
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
	if dc.DSN != "" {
		return dc.DSN
	}
	// Build a DSN from components (Postgres)
	return dc.BuildDSN()
}

// BuildDSN builds a postgres DSN from components. It will prefer explicit fields
// and fall back to sensible defaults where appropriate.
func (dc *DatabaseConfig) BuildDSN() string {
	// minimal builder: user/password@host:port/dbname?sslmode=...
	userPart := ""
	if dc.User != "" {
		userPart = dc.User
		if dc.Password != "" {
			userPart += ":" + dc.Password
		}
		userPart += "@"
	}

	host := dc.Host
	if host == "" {
		host = "localhost"
	}
	port := dc.Port
	if port == 0 {
		port = 5432
	}
	ssl := dc.SSLMode
	if ssl == "" {
		ssl = "disable"
	}

	return fmt.Sprintf("postgres://%s%s:%d/%s?sslmode=%s", userPart, host, port, dc.DBName, ssl)
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

// Sanitize returns a copy of the Config with secret fields redacted.
// It is safe to call before logging or including config in diagnostic output.
func (c *Config) Sanitize() *Config {
	out := &Config{
		Default:   c.Default,
		Databases: make(map[string]*DatabaseConfig, len(c.Databases)),
	}

	for k, db := range c.Databases {
		if db == nil {
			out.Databases[k] = nil
			continue
		}
		copyDB := *db
		if copyDB.DSN != "" {
			copyDB.DSN = "[redacted]"
		}
		if copyDB.Password != "" {
			copyDB.Password = "[redacted]"
		}
		// leave other non-secret fields intact
		out.Databases[k] = &copyDB
	}

	return out
}

// ConfigSummary returns a compact diagnostic map suitable for logging.
// It intentionally avoids including secret material.
func (c *Config) ConfigSummary() map[string]any {
	dbs := make(map[string]map[string]any)
	for name, db := range c.Databases {
		if db == nil {
			dbs[name] = map[string]any{"present": false}
			continue
		}
		dbs[name] = map[string]any{
			"driver":       db.Driver,
			"host":         db.Host,
			"port":         db.Port,
			"dbname":       db.DBName,
			"has_dsn":      db.DSN != "",
			"has_password": db.Password != "",
		}
	}

	return map[string]any{
		"default":   c.Default,
		"databases": dbs,
	}
}
