package migrate

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.False(t, cfg.AutoMigrate, "AutoMigrate should default to false for safety")
	assert.Empty(t, cfg.Dir)
	assert.False(t, cfg.UseEmbed)
	assert.Equal(t, "schema_migrations", cfg.Table)
	assert.Equal(t, 15*time.Second, cfg.LockTimeout)
	assert.False(t, cfg.Verbose)
}

func TestConfigOptions(t *testing.T) {
	t.Run("WithDir", func(t *testing.T) {
		cfg := DefaultConfig()
		WithDir("/path/to/migrations")(cfg)

		assert.Equal(t, "/path/to/migrations", cfg.Dir)
		assert.False(t, cfg.UseEmbed, "Dir should disable UseEmbed")
	})

	t.Run("WithEmbed", func(t *testing.T) {
		cfg := DefaultConfig()
		WithEmbed()(cfg)

		assert.True(t, cfg.UseEmbed)
		assert.Empty(t, cfg.Dir, "UseEmbed should clear Dir")
	})

	t.Run("WithTable", func(t *testing.T) {
		cfg := DefaultConfig()
		WithTable("custom_migrations")(cfg)

		assert.Equal(t, "custom_migrations", cfg.Table)
	})

	t.Run("WithLockTimeout", func(t *testing.T) {
		cfg := DefaultConfig()
		WithLockTimeout(30 * time.Second)(cfg)

		assert.Equal(t, 30*time.Second, cfg.LockTimeout)
	})

	t.Run("WithVerbose", func(t *testing.T) {
		cfg := DefaultConfig()
		WithVerbose()(cfg)

		assert.True(t, cfg.Verbose)
	})

	t.Run("WithAutoMigrate", func(t *testing.T) {
		cfg := DefaultConfig()
		WithAutoMigrate()(cfg)

		assert.True(t, cfg.AutoMigrate)
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("Valid config with Dir", func(t *testing.T) {
		cfg := &Config{
			AutoMigrate: false,
			Dir:         "/path/to/migrations",
			UseEmbed:    false,
			Table:       "schema_migrations",
			LockTimeout: 15 * time.Second,
		}

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid config with UseEmbed", func(t *testing.T) {
		cfg := &Config{
			AutoMigrate: false,
			Dir:         "",
			UseEmbed:    true,
			Table:       "schema_migrations",
			LockTimeout: 15 * time.Second,
		}

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid: both Dir and UseEmbed set", func(t *testing.T) {
		cfg := &Config{
			AutoMigrate: false,
			Dir:         "/path/to/migrations",
			UseEmbed:    true,
			Table:       "schema_migrations",
			LockTimeout: 15 * time.Second,
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidConfig)
	})

	t.Run("Invalid: AutoMigrate enabled without source", func(t *testing.T) {
		cfg := &Config{
			AutoMigrate: true,
			Dir:         "",
			UseEmbed:    false,
			Table:       "schema_migrations",
			LockTimeout: 15 * time.Second,
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNoMigrationSource)
	})

	t.Run("Invalid: empty table name", func(t *testing.T) {
		cfg := &Config{
			AutoMigrate: false,
			Dir:         "/path/to/migrations",
			UseEmbed:    false,
			Table:       "",
			LockTimeout: 15 * time.Second,
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidConfig)
	})

	t.Run("Invalid: negative lock timeout", func(t *testing.T) {
		cfg := &Config{
			AutoMigrate: false,
			Dir:         "/path/to/migrations",
			UseEmbed:    false,
			Table:       "schema_migrations",
			LockTimeout: -1 * time.Second,
		}

		err := cfg.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidConfig)
	})

	t.Run("Valid: no source for library usage", func(t *testing.T) {
		cfg := &Config{
			AutoMigrate: false,
			Dir:         "",
			UseEmbed:    false,
			Table:       "schema_migrations",
			LockTimeout: 15 * time.Second,
		}

		err := cfg.Validate()
		assert.NoError(t, err, "No source is OK when AutoMigrate is false")
	})
}

func TestNewConfigFromViper(t *testing.T) {
	t.Run("Load from viper with defaults", func(t *testing.T) {
		v := viper.New()
		cfg, err := NewConfig(v)

		require.NoError(t, err)
		assert.False(t, cfg.AutoMigrate)
		assert.Empty(t, cfg.Dir)
		assert.False(t, cfg.UseEmbed)
		assert.Equal(t, "schema_migrations", cfg.Table)
		assert.Equal(t, 15*time.Second, cfg.LockTimeout)
	})

	t.Run("Load from viper with custom values", func(t *testing.T) {
		v := viper.New()
		v.Set("dbx.migrate.auto_migrate", true)
		v.Set("dbx.migrate.dir", "/custom/migrations")
		v.Set("dbx.migrate.table", "custom_table")
		v.Set("dbx.migrate.lock_timeout", "30s")
		v.Set("dbx.migrate.verbose", true)

		cfg, err := NewConfig(v)

		require.NoError(t, err)
		assert.True(t, cfg.AutoMigrate)
		assert.Equal(t, "/custom/migrations", cfg.Dir)
		assert.Equal(t, "custom_table", cfg.Table)
		assert.Equal(t, 30*time.Second, cfg.LockTimeout)
		assert.True(t, cfg.Verbose)
	})

	t.Run("Load from environment variables (legacy)", func(t *testing.T) {
		v := viper.New()

		// Simulate legacy environment variables
		v.Set("dbx.migrate.auto_migrate", false)
		v.Set("dbx.migrate.use_embed", true)
		v.Set("dbx.migrate.table", "env_migrations")

		cfg, err := NewConfig(v)

		require.NoError(t, err)
		assert.False(t, cfg.AutoMigrate)
		assert.True(t, cfg.UseEmbed)
		assert.Equal(t, "env_migrations", cfg.Table)
	})

	t.Run("Load from unified configuration (databases.primary)", func(t *testing.T) {
		v := viper.New()
		v.Set("databases.primary.auto_migrate", true)
		v.Set("databases.primary.migration_source", "file://./migrations")
		v.Set("databases.primary.migration_table", "unified_migrations")
		v.Set("databases.primary.migration_lock_timeout", "30s")
		v.Set("databases.primary.migration_verbose", true)

		cfg, err := NewConfig(v)

		require.NoError(t, err)
		assert.True(t, cfg.AutoMigrate)
		assert.Equal(t, "./migrations", cfg.Dir)
		assert.False(t, cfg.UseEmbed)
		assert.Equal(t, "unified_migrations", cfg.Table)
		assert.Equal(t, 30*time.Second, cfg.LockTimeout)
		assert.True(t, cfg.Verbose)
	})

	t.Run("Load from unified configuration with embed source", func(t *testing.T) {
		v := viper.New()
		v.Set("databases.primary.auto_migrate", false)
		v.Set("databases.primary.migration_source", "embed://")
		v.Set("databases.primary.migration_table", "embedded_migrations")

		cfg, err := NewConfig(v)

		require.NoError(t, err)
		assert.False(t, cfg.AutoMigrate)
		assert.True(t, cfg.UseEmbed)
		assert.Empty(t, cfg.Dir)
		assert.Equal(t, "embedded_migrations", cfg.Table)
	})
}

func TestConfigClone(t *testing.T) {
	original := &Config{
		AutoMigrate: true,
		Dir:         "/path/to/migrations",
		UseEmbed:    false,
		Table:       "schema_migrations",
		LockTimeout: 30 * time.Second,
		Verbose:     true,
	}

	cloned := original.Clone()

	// Verify all fields are copied
	assert.Equal(t, original.AutoMigrate, cloned.AutoMigrate)
	assert.Equal(t, original.Dir, cloned.Dir)
	assert.Equal(t, original.UseEmbed, cloned.UseEmbed)
	assert.Equal(t, original.Table, cloned.Table)
	assert.Equal(t, original.LockTimeout, cloned.LockTimeout)
	assert.Equal(t, original.Verbose, cloned.Verbose)

	// Verify they are different instances
	cloned.AutoMigrate = false
	assert.True(t, original.AutoMigrate, "Original should not be affected by clone modification")
}

func TestConfigApply(t *testing.T) {
	cfg := DefaultConfig()

	cfg.Apply(
		WithDir("/custom/path"),
		WithTable("custom_table"),
		WithVerbose(),
		WithAutoMigrate(),
	)

	assert.Equal(t, "/custom/path", cfg.Dir)
	assert.Equal(t, "custom_table", cfg.Table)
	assert.True(t, cfg.Verbose)
	assert.True(t, cfg.AutoMigrate)
}

func TestErrorHelpers(t *testing.T) {
	t.Run("IsNoChange", func(t *testing.T) {
		assert.True(t, IsNoChange(ErrNoChange))
		assert.False(t, IsNoChange(ErrInvalidConfig))
	})

	t.Run("IsNilVersion", func(t *testing.T) {
		assert.True(t, IsNilVersion(ErrNilVersion))
		assert.False(t, IsNilVersion(ErrNoChange))
	})

	t.Run("IsLocked", func(t *testing.T) {
		assert.True(t, IsLocked(ErrLocked))
		assert.False(t, IsLocked(ErrNoChange))
	})
}
