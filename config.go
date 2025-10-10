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

	return nil
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
}