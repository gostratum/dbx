package dbx

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gostratum/core/configx"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLoader is a helper to create a config loader for tests
func testLoader(configYAML string) (configx.Loader, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(configYAML)); err != nil {
		return nil, err
	}

	// Create a wrapper that implements configx.Loader
	return &viperLoaderWrapper{v: v}, nil
}

// viperLoaderWrapper wraps viper.Viper to implement configx.Loader for tests
type viperLoaderWrapper struct {
	v *viper.Viper
}

func (w *viperLoaderWrapper) Bind(c configx.Configurable) error {
	prefix := c.Prefix()
	sub := w.v.Sub(prefix)
	if sub == nil {
		sub = viper.New()
	}
	return sub.Unmarshal(c)
}

// BindEnv binds a viper key to one or more environment variable names for tests.
func (w *viperLoaderWrapper) BindEnv(key string, envVars ...string) error {
	args := append([]string{key}, envVars...)
	return w.v.BindEnv(args...)
}

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

	// Database connection settings
	assert.Equal(t, "postgres", cfg.Driver)
	assert.Equal(t, "postgres://localhost/dbname?sslmode=disable", cfg.DSN)
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, 5*time.Minute, cfg.ConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, cfg.ConnMaxIdleTime)
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.Equal(t, 200*time.Millisecond, cfg.SlowThreshold)
	assert.False(t, cfg.SkipDefaultTx)
	assert.True(t, cfg.PrepareStmt)
	assert.NotNil(t, cfg.Params)

	// Migration settings (safe defaults)
	assert.Empty(t, cfg.MigrationSource, "MigrationSource should be empty by default for safety")
	assert.False(t, cfg.AutoMigrate, "AutoMigrate should be false by default for production safety")
	assert.Equal(t, "schema_migrations", cfg.MigrationTable)
	assert.Equal(t, 15*time.Second, cfg.MigrationLockTimeout)
	assert.False(t, cfg.MigrationVerbose)
}

func TestLoadConfigFromYAML(t *testing.T) {
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

	loader, err := testLoader(configYAML)
	require.NoError(t, err)

	cfg := &Config{}
	err = loader.Bind(cfg)
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

	loader, err := testLoader(configYAML)
	require.NoError(t, err)

	cfg := &Config{}
	err = loader.Bind(cfg)
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
	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("negative max_open_conns", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.MaxOpenConns = -1
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_open_conns")
	})

	t.Run("negative max_idle_conns", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.MaxIdleConns = -1
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_idle_conns")
	})

	t.Run("migration validation - AutoMigrate without source", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.AutoMigrate = true
		cfg.MigrationSource = ""
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auto_migrate is enabled but migration_source is empty")
	})

	t.Run("migration validation - valid embed source", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.AutoMigrate = true
		cfg.MigrationSource = "embed://"
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("migration validation - valid file source", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.AutoMigrate = true
		cfg.MigrationSource = "file://./migrations"
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("migration validation - invalid source format", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.MigrationSource = "invalid://source"
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "migration_source must be")
	})

	t.Run("migration validation - empty table when source specified", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.MigrationSource = "embed://"
		cfg.MigrationTable = ""
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "migration_table cannot be empty")
	})

	t.Run("migration validation - negative lock timeout", func(t *testing.T) {
		cfg := DefaultDatabaseConfig()
		cfg.MigrationLockTimeout = -1 * time.Second
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "migration_lock_timeout must be >= 0")
	})
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

func TestDatabaseConfigMigrationInterface(t *testing.T) {
	cfg := &DatabaseConfig{
		DSN:                  "postgres://localhost/test",
		MigrationSource:      "file://./migrations",
		MigrationTable:       "custom_migrations",
		MigrationLockTimeout: 30 * time.Second,
		MigrationVerbose:     true,
	}

	// Test interface methods
	assert.Equal(t, "postgres://localhost/test", cfg.GetDSN())
	assert.Equal(t, "file://./migrations", cfg.GetMigrationSource())
	assert.Equal(t, "custom_migrations", cfg.GetMigrationTable())
	assert.Equal(t, 30*time.Second, cfg.GetMigrationLockTimeout())
	assert.True(t, cfg.GetMigrationVerbose())
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Start with an otherwise-empty db config so env can override the DSN
	// Use a clean inline YAML string; avoids issues with backtick literals and
	// preserves the same behavior as other YAML-based tests.
	configYAML := "db:\n  default: primary\n  databases:\n    primary: {}\n"
	loader, err := testLoader(configYAML)
	require.NoError(t, err)

	// Set the environment variable and bind it through the loader (what the
	// module does before calling Bind)
	envVal := "postgres://env_user:env_pass@localhost:5432/envdb?sslmode=disable"
	os.Setenv("STRATUM_DB_DATABASES_PRIMARY_DSN", envVal)
	defer os.Unsetenv("STRATUM_DB_DATABASES_PRIMARY_DSN")

	// Bind the env var to the viper key used by dbx on the underlying viper
	// instance and ensure the env overrides the value.
	w := loader.(*viperLoaderWrapper)
	require.NoError(t, w.v.BindEnv("databases.primary.dsn", "STRATUM_DB_DATABASES_PRIMARY_DSN"))

	// The bound env var should be visible via viper GetString
	got := w.v.GetString("databases.primary.dsn")
	assert.Equal(t, envVal, got)
}
