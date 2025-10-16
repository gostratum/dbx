# Changelog

All notable changes to the DBX module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **BREAKING**: Configuration now uses `core/configx.Loader` instead of direct `viper.Viper` dependency
- **BREAKING**: Removed `LoadConfig(v *viper.Viper) (*Config, error)` function
- **BREAKING**: Health checks now use `core.Registry` instead of custom `HealthRegistry` interface
- **BREAKING**: Removed custom `HealthCheck` struct and `HealthCheckFunc` type
- Environment variables now require `STRATUM_` prefix (handled by `core/configx`)
- Implemented `configx.Configurable` interface with `Prefix()` method
- Updated health checks to implement `core.Check` interface

### Added
- Added struct tags with `default:` values for automatic default configuration
- New `dbCheck` struct implementing `core.Check` interface
- Comprehensive refactoring documentation in `REFACTORING_SUMMARY.md`
- Updated README with new configuration patterns
- Test helpers for new configuration loading pattern

### Fixed
- Configuration loading is now more consistent with other Gostratum modules
- Better separation of concerns between configuration and module logic

### Migration Guide

#### For users calling `LoadConfig()` directly:

**Before**:
```go
dbConfig, err := dbx.LoadConfig(viper.GetViper())
```

**After**:
```go
loader := configx.New()
dbConfig := dbx.DefaultConfig()
err := loader.Bind(dbConfig)
```

#### For users implementing custom health registries:

**Before**:
```go
type MyRegistry struct{}
func (r *MyRegistry) RegisterReadinessCheck(check *dbx.HealthCheck) error { ... }
func (r *MyRegistry) RegisterLivenessCheck(check *dbx.HealthCheck) error { ... }

checker := dbx.NewHealthChecker(connections, myRegistry)
```

**After**:
```go
registry := core.NewHealthRegistry()
checker := dbx.NewHealthChecker(connections, registry)
checker.RegisterHealthChecks()
```

#### For users using the Fx module pattern:

**No changes required!** The module API remains the same:
```go
app := core.New(
    dbx.Module(
        dbx.WithDefault("primary"),
        dbx.WithHealthChecks(),
    ),
)
```

#### Environment Variables

**Before**: `DB_DEFAULT=primary`  
**After**: `STRATUM_DB_DEFAULT=primary`

All environment variables now require the `STRATUM_` prefix.

## Previous Versions

See git history for changes prior to this version.
