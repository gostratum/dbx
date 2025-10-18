package dbx

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/gostratum/core"
	"github.com/gostratum/core/configx"
	"github.com/gostratum/core/logx"
	"github.com/gostratum/dbx/migrate"
	"github.com/gostratum/metricsx"
	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connections represents a map of database connections
type Connections map[string]*gorm.DB

// Provider provides the default database connection
type Provider struct {
	connections Connections
	defaultName string
}

// Get returns the default database connection
func (p *Provider) Get() *gorm.DB {
	if p.defaultName == "" {
		// Return first available connection if no default set
		for _, db := range p.connections {
			return db
		}
		return nil
	}
	return p.connections[p.defaultName]
}

// GetByName returns a database connection by name
func (p *Provider) GetByName(name string) *gorm.DB {
	return p.connections[name]
}

// GetConnections returns all database connections
func (p *Provider) GetConnections() Connections {
	return p.connections
}

// Option configures the dbx module
type Option func(*moduleConfig)

// moduleConfig holds the module configuration
type moduleConfig struct {
	defaultName   string
	autoMigrate   []any
	migrationsFS  fs.FS
	migrationsDir string
	runMigrations bool
	gormConfig    *gorm.Config
	healthChecks  bool
	// New golang-migrate support
	useGolangMigrate      bool
	golangMigrateUseEmbed bool
	golangMigrateDir      string
}

// WithDefault sets the default database connection name
func WithDefault(name string) Option {
	return func(cfg *moduleConfig) {
		cfg.defaultName = name
	}
}

// WithAutoMigrate enables auto-migration for the given models
func WithAutoMigrate(models ...any) Option {
	return func(cfg *moduleConfig) {
		cfg.autoMigrate = append(cfg.autoMigrate, models...)
	}
}

// WithMigrationsFS sets the filesystem for SQL migrations
func WithMigrationsFS(filesystem fs.FS, dir ...string) Option {
	return func(cfg *moduleConfig) {
		cfg.migrationsFS = filesystem
		if len(dir) > 0 {
			cfg.migrationsDir = dir[0]
		} else {
			cfg.migrationsDir = "migrations"
		}
	}
}

// WithRunMigrations enables running migrations at startup
func WithRunMigrations() Option {
	return func(cfg *moduleConfig) {
		cfg.runMigrations = true
	}
}

// WithGormConfig sets custom GORM configuration
func WithGormConfig(gormCfg *gorm.Config) Option {
	return func(cfg *moduleConfig) {
		cfg.gormConfig = gormCfg
	}
}

// WithHealthChecks enables health check registration
func WithHealthChecks() Option {
	return func(cfg *moduleConfig) {
		cfg.healthChecks = true
	}
}

// WithGolangMigrate enables golang-migrate based migrations at startup
// This provides first-class migration support using golang-migrate library
func WithGolangMigrate() Option {
	return func(cfg *moduleConfig) {
		cfg.useGolangMigrate = true
	}
}

// WithGolangMigrateEmbed enables golang-migrate with embedded migrations
func WithGolangMigrateEmbed() Option {
	return func(cfg *moduleConfig) {
		cfg.useGolangMigrate = true
		cfg.golangMigrateUseEmbed = true
	}
}

// WithGolangMigrateDir sets the directory for golang-migrate filesystem migrations
func WithGolangMigrateDir(dir string) Option {
	return func(cfg *moduleConfig) {
		cfg.useGolangMigrate = true
		cfg.golangMigrateDir = dir
	}
}

// Module creates the dbx Fx module
func Module(opts ...Option) fx.Option {
	cfg := &moduleConfig{
		defaultName:   "",
		autoMigrate:   make([]any, 0),
		runMigrations: false,
		healthChecks:  true, // enabled by default
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	return fx.Module("dbx",
		// Provide database connections
		fx.Provide(
			func(loader configx.Loader, logger logx.Logger) (Connections, error) {
				return newConnections(loader, logger, cfg)
			},
		),
		// Provide the default database connection
		fx.Provide(
			func(connections Connections) (*Provider, *gorm.DB, error) {
				provider := &Provider{
					connections: connections,
					defaultName: cfg.defaultName,
				}

				defaultDB := provider.Get()
				if defaultDB == nil {
					return nil, nil, fmt.Errorf("no default database connection available")
				}

				return provider, defaultDB, nil
			},
		),
		// Provide migration runner
		fx.Provide(
			func(logger logx.Logger, connections Connections) *MigrationRunner {
				var opts []MigrationOption

				if len(cfg.autoMigrate) > 0 {
					opts = append(opts, withAutoMigrate(cfg.autoMigrate...))
				}

				if cfg.migrationsFS != nil {
					opts = append(opts, withMigrationsFS(cfg.migrationsFS, cfg.migrationsDir))
				}

				return NewMigrationRunner(logger, connections, opts...)
			},
		),
		// Provide health checker if enabled
		fx.Provide(
			fx.Annotated{
				Target: func(connections Connections, registry core.Registry) *HealthChecker {
					if !cfg.healthChecks {
						return nil
					}
					return NewHealthChecker(connections, registry)
				},
				Group: "health_checkers",
			},
		),
		// Register metrics plugin if metricsx is available
		fx.Invoke(
			func(lc fx.Lifecycle, params struct {
				fx.In
				Connections Connections
				Logger      logx.Logger
				Metrics     metricsx.Metrics `optional:"true"`
			}) {
				if params.Metrics == nil {
					return
				}

				params.Logger.Info("dbx: enabling database metrics")

				// Channel to stop metrics collection
				stopChan := make(chan struct{})

				for name, db := range params.Connections {
					// Register metrics plugin
					plugin := NewMetricsPlugin(params.Metrics)
					if err := db.Use(plugin); err != nil {
						params.Logger.Error("dbx: failed to register metrics plugin",
							logx.String("database", name),
							logx.Err(err),
						)
						continue
					}

					// Start connection pool metrics collector with context
					ConnectionPoolMetricsWithContext(params.Metrics, db, name, stopChan)

					params.Logger.Info("dbx: metrics enabled for database", logx.String("database", name))
				}

				// Add lifecycle hook to stop metrics collection
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						params.Logger.Info("dbx: stopping metrics collection")
						close(stopChan)
						return nil
					},
				})
			},
		),
		// Lifecycle hooks
		fx.Invoke(func(lc fx.Lifecycle, logger logx.Logger, loader configx.Loader, connections Connections, migrationRunner *MigrationRunner, healthChecker *HealthChecker) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					logger.Info("Starting dbx module")

					// Test all connections
					for name, db := range connections {
						if err := testConnection(ctx, name, db, logger); err != nil {
							return err
						}
					}

					// Run golang-migrate migrations if enabled
					if cfg.useGolangMigrate {
						if err := runGolangMigrations(ctx, logger, loader, cfg); err != nil {
							return fmt.Errorf("golang-migrate migration failed: %w", err)
						}
					}

					// Run GORM migrations if enabled (for backward compatibility)
					if cfg.runMigrations {
						if err := migrationRunner.RunMigrations(); err != nil {
							return fmt.Errorf("GORM migration failed: %w", err)
						}
					}

					// Register health checks if enabled
					if cfg.healthChecks && healthChecker != nil {
						if err := healthChecker.RegisterHealthChecks(); err != nil {
							logger.Error("Failed to register health checks", logx.Err(err))
						} else {
							logger.Info("Health checks registered", logx.Int("databases", len(connections)))
						}
					}

					logger.Info("dbx module started successfully")
					return nil
				},
				OnStop: func(ctx context.Context) error {
					logger.Info("Stopping dbx module")

					// Close all connections
					for name, db := range connections {
						if sqlDB, err := db.DB(); err == nil {
							if err := sqlDB.Close(); err != nil {
								logger.Error("Failed to close database connection",
									logx.String("database", name),
									logx.Err(err))
							} else {
								logger.Info("Database connection closed", logx.String("database", name))
							}
						}
					}

					logger.Info("dbx module stopped")
					return nil
				},
			})
		}),
	)
}

// newConnections creates database connections based on configuration
func newConnections(loader configx.Loader, logger logx.Logger, cfg *moduleConfig) (Connections, error) {
	// Load configuration using core configx pattern
	dbConfig := DefaultConfig()
	// Bind DB env variables to viper keys for databases so that env-provided
	// DSNs are available during Bind. Keep this DB-specific behavior here
	// to avoid leaking database concerns into configx.
	for name := range dbConfig.Databases {
		key := fmt.Sprintf("databases.%s.dsn", name)
		env := fmt.Sprintf("DB_DATABASES_%s_DSN", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
		envStratum := fmt.Sprintf("STRATUM_%s", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
		_ = loader.BindEnv(key, env, envStratum)
	}

	if err := loader.Bind(dbConfig); err != nil {
		return nil, fmt.Errorf("failed to load database configuration: %w", err)
	}

	// Validate configuration
	if err := dbConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	connections := make(Connections)

	for name, dbCfg := range dbConfig.Databases {
		db, err := createConnection(name, dbCfg, logger, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create connection for database %s: %w", name, err)
		}
		connections[name] = db
	}

	return connections, nil
}

// createConnection creates a single database connection
func createConnection(name string, dbCfg *DatabaseConfig, logger logx.Logger, cfg *moduleConfig) (*gorm.DB, error) {
	logger.Info("Creating database connection", logx.String("database", name), logx.String("driver", dbCfg.Driver))

	// Create GORM config
	gormCfg := &gorm.Config{
		SkipDefaultTransaction: dbCfg.SkipDefaultTx,
		PrepareStmt:            dbCfg.PrepareStmt,
		Logger:                 NewGormLogger(logger, dbCfg.LogLevel, dbCfg.SlowThreshold),
	}

	// Override with custom config if provided
	if cfg.gormConfig != nil {
		gormCfg = cfg.gormConfig
		// But still use our logger
		gormCfg.Logger = NewGormLogger(logger, dbCfg.LogLevel, dbCfg.SlowThreshold)
	}

	// Create database connection based on driver
	var dialector gorm.Dialector
	switch dbCfg.Driver {
	case "postgres":
		dialector = postgres.Open(dbCfg.GetDSN())
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", dbCfg.Driver)
	}

	// Open connection
	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(dbCfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbCfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(dbCfg.ConnMaxIdleTime)

	logger.Info("Database connection created successfully", logx.String("database", name))
	return db, nil
}

// testConnection tests a database connection
func testConnection(ctx context.Context, name string, db *gorm.DB, logger logx.Logger) error {
	logger.Info("Testing database connection", logx.String("database", name))

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying DB for %s: %w", name, err)
	}

	// Test connection with timeout
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(testCtx); err != nil {
		return fmt.Errorf("database ping failed for %s: %w", name, err)
	}

	logger.Info("Database connection test successful", logx.String("database", name))
	return nil
}

// runGolangMigrations runs golang-migrate based migrations
func runGolangMigrations(ctx context.Context, logger logx.Logger, loader configx.Loader, cfg *moduleConfig) error {
	logger.Info("Running golang-migrate migrations")

	// Load database configuration to get migration settings
	dbConfig := DefaultConfig()
	if err := loader.Bind(dbConfig); err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}

	// Get the default database configuration (which contains migration settings)
	defaultDB, err := dbConfig.GetDefaultDatabase()
	if err != nil {
		return fmt.Errorf("failed to get default database config: %w", err)
	}

	// Override migration source with module config if specified
	migrationSource := defaultDB.MigrationSource
	if cfg.golangMigrateUseEmbed {
		migrationSource = "embed://"
	} else if cfg.golangMigrateDir != "" {
		migrationSource = fmt.Sprintf("file://%s", cfg.golangMigrateDir)
	}

	// Check if migrations are enabled
	autoMigrate := defaultDB.AutoMigrate
	if migrationSource == "" {
		logger.Info("Migration source not configured, skipping migrations")
		return nil
	}

	if !autoMigrate {
		logger.Info("AutoMigrate is disabled, skipping migrations")
		return nil
	}

	logger.Info("Migration settings loaded",
		logx.String("source", migrationSource),
		logx.Bool("auto_migrate", autoMigrate),
		logx.String("table", defaultDB.MigrationTable),
		logx.Duration("lock_timeout", defaultDB.MigrationLockTimeout),
	)

	// Build migration options based on source type
	var opts []migrate.Option

	if migrationSource == "embed://" {
		opts = append(opts, migrate.WithEmbed())
	} else if len(migrationSource) > 7 && migrationSource[:7] == "file://" {
		dir := migrationSource[7:] // Remove "file://" prefix
		opts = append(opts, migrate.WithDir(dir))
	} else {
		return fmt.Errorf("invalid migration_source format: %s (use 'embed://' or 'file://path')", migrationSource)
	}

	// Add other migration options
	opts = append(opts,
		migrate.WithTable(defaultDB.MigrationTable),
		migrate.WithLockTimeout(defaultDB.MigrationLockTimeout),
	)

	if defaultDB.MigrationVerbose {
		opts = append(opts, migrate.WithVerbose())
	}

	// Run migrations using the integrated database config
	logger.Info("Applying pending migrations...")
	if err := migrate.UpFromDatabaseConfig(ctx, defaultDB); err != nil {
		if migrate.IsNoChange(err) {
			logger.Info("No pending migrations to apply")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	logger.Info("Migrations applied successfully")
	return nil
}
