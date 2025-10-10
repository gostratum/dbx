package dbx

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "primary", cfg.Default)
	assert.Len(t, cfg.Databases, 1)

	primaryDB := cfg.Databases["primary"]
	assert.Equal(t, "postgres", primaryDB.Driver)
	assert.Equal(t, 25, primaryDB.MaxOpenConns)
	assert.Equal(t, 5, primaryDB.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, primaryDB.ConnMaxLifetime)
	assert.Equal(t, "warn", primaryDB.LogLevel)
	assert.Equal(t, 200*time.Millisecond, primaryDB.SlowThreshold)
}

func TestDefaultDatabaseConfig(t *testing.T) {
	cfg := DefaultDatabaseConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "postgres", cfg.Driver)
	assert.True(t, strings.Contains(cfg.DSN, "postgres://"))
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.False(t, cfg.SkipDefaultTx)
	assert.True(t, cfg.PrepareStmt)
}

func TestLoadConfigFromYAML(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")

	configYAML := `
db:
  default: test
  databases:
    test:
      driver: postgres
      dsn: "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
      max_open_conns: 50
      max_idle_conns: 10
      conn_max_lifetime: 10m
      conn_max_idle_time: 5m
      log_level: info
      slow_threshold: 500ms
      skip_default_tx: true
      prepare_stmt: false
`

	err := v.ReadConfig(strings.NewReader(configYAML))
	require.NoError(t, err)

	cfg, err := LoadConfig(v)
	require.NoError(t, err)

	assert.Equal(t, "test", cfg.Default)
	assert.Len(t, cfg.Databases, 1)

	testDB := cfg.Databases["test"]
	assert.Equal(t, "postgres", testDB.Driver)
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb?sslmode=disable", testDB.DSN)
	assert.Equal(t, 50, testDB.MaxOpenConns)
	assert.Equal(t, 10, testDB.MaxIdleConns)
	assert.Equal(t, 10*time.Minute, testDB.ConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, testDB.ConnMaxIdleTime)
	assert.Equal(t, "info", testDB.LogLevel)
	assert.Equal(t, 500*time.Millisecond, testDB.SlowThreshold)
	assert.True(t, testDB.SkipDefaultTx)
	assert.False(t, testDB.PrepareStmt)
}

func TestLoadConfigMultipleDatabases(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")

	configYAML := `
db:
  default: primary
  databases:
    primary:
      driver: postgres
      dsn: "postgres://localhost:5432/app_db"
      max_open_conns: 25
    analytics:
      driver: postgres
      dsn: "postgres://localhost:5432/analytics_db"
      max_open_conns: 10
      log_level: silent
`

	err := v.ReadConfig(strings.NewReader(configYAML))
	require.NoError(t, err)

	cfg, err := LoadConfig(v)
	require.NoError(t, err)

	assert.Equal(t, "primary", cfg.Default)
	assert.Len(t, cfg.Databases, 2)

	primaryDB := cfg.Databases["primary"]
	assert.Equal(t, "postgres://localhost:5432/app_db", primaryDB.DSN)
	assert.Equal(t, 25, primaryDB.MaxOpenConns)

	analyticsDB := cfg.Databases["analytics"]
	assert.Equal(t, "postgres://localhost:5432/analytics_db", analyticsDB.DSN)
	assert.Equal(t, 10, analyticsDB.MaxOpenConns)
	assert.Equal(t, "silent", analyticsDB.LogLevel)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &Config{
				Default: "primary",
				Databases: map[string]*DatabaseConfig{
					"primary": {
						Driver: "postgres",
						DSN:    "postgres://localhost/test",
					},
				},
			},
			expectError: false,
		},
		{
			name:        "empty databases",
			config:      &Config{Databases: map[string]*DatabaseConfig{}},
			expectError: true,
			errorMsg:    "no databases configured",
		},
		{
			name: "default database not found",
			config: &Config{
				Default: "nonexistent",
				Databases: map[string]*DatabaseConfig{
					"primary": {
						Driver: "postgres",
						DSN:    "postgres://localhost/test",
					},
				},
			},
			expectError: true,
			errorMsg:    "default database 'nonexistent' not found",
		},
		{
			name: "invalid database config - no driver",
			config: &Config{
				Default: "primary",
				Databases: map[string]*DatabaseConfig{
					"primary": {
						DSN: "postgres://localhost/test",
					},
				},
			},
			expectError: true,
			errorMsg:    "driver is required",
		},
		{
			name: "invalid database config - no DSN",
			config: &Config{
				Default: "primary",
				Databases: map[string]*DatabaseConfig{
					"primary": {
						Driver: "postgres",
					},
				},
			},
			expectError: true,
			errorMsg:    "dsn is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDatabaseConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *DatabaseConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &DatabaseConfig{
				Driver:          "postgres",
				DSN:             "postgres://localhost/test",
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			},
			expectError: false,
		},
		{
			name: "negative max open conns",
			config: &DatabaseConfig{
				Driver:       "postgres",
				DSN:          "postgres://localhost/test",
				MaxOpenConns: -1,
			},
			expectError: true,
			errorMsg:    "max_open_conns must be >= 0",
		},
		{
			name: "negative max idle conns",
			config: &DatabaseConfig{
				Driver:       "postgres",
				DSN:          "postgres://localhost/test",
				MaxIdleConns: -1,
			},
			expectError: true,
			errorMsg:    "max_idle_conns must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetDefaultDatabase(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		expectedDSN string
	}{
		{
			name: "explicit default",
			config: &Config{
				Default: "primary",
				Databases: map[string]*DatabaseConfig{
					"primary":   {DSN: "postgres://localhost/primary"},
					"secondary": {DSN: "postgres://localhost/secondary"},
				},
			},
			expectError: false,
			expectedDSN: "postgres://localhost/primary",
		},
		{
			name: "no default set, returns first",
			config: &Config{
				Databases: map[string]*DatabaseConfig{
					"first": {DSN: "postgres://localhost/first"},
				},
			},
			expectError: false,
			expectedDSN: "postgres://localhost/first",
		},
		{
			name: "no databases",
			config: &Config{
				Databases: map[string]*DatabaseConfig{},
			},
			expectError: true,
		},
		{
			name: "default not found",
			config: &Config{
				Default: "nonexistent",
				Databases: map[string]*DatabaseConfig{
					"primary": {DSN: "postgres://localhost/primary"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := tt.config.GetDefaultDatabase()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, db)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, db)
				assert.Equal(t, tt.expectedDSN, db.DSN)
			}
		})
	}
}
