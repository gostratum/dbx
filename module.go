package dbx

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/zap"
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
	defaultName    string
	autoMigrate    []interface{}
	migrationsFS   fs.FS
	migrationsDir  string
	runMigrations  bool
	gormConfig     *gorm.Config
	healthChecks   bool
}

// WithDefault sets the default database connection name
func WithDefault(name string) Option {
	return func(cfg *moduleConfig) {
		cfg.defaultName = name
	}
}

// WithAutoMigrate enables auto-migration for the given models  
func WithAutoMigrate(models ...interface{}) Option {
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

// Module creates the dbx Fx module
func Module(opts ...Option) fx.Option {
	cfg := &moduleConfig{
		defaultName:   "",
		autoMigrate:   make([]interface{}, 0),
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
			func(v *viper.Viper, logger *zap.Logger) (Connections, error) {
				return newConnections(v, logger, cfg)
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
			func(logger *zap.Logger, connections Connections) *MigrationRunner {
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
				Target: func(connections Connections) *HealthChecker {
					if !cfg.healthChecks {
						return nil
					}
					// For now, return a health checker without registry integration
					// This can be enhanced when the actual core.Registry interface is available
					return NewHealthChecker(connections, nil)
				},
				Group: "health_checkers",
			},
		),
		// Lifecycle hooks
		fx.Invoke(func(lc fx.Lifecycle, logger *zap.Logger, connections Connections, migrationRunner *MigrationRunner, healthChecker *HealthChecker) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					logger.Info("Starting dbx module")
					
					// Test all connections
					for name, db := range connections {
						if err := testConnection(ctx, name, db, logger); err != nil {
							return err
						}
					}
					
					// Run migrations if enabled
					if cfg.runMigrations {
						if err := migrationRunner.RunMigrations(); err != nil {
							return fmt.Errorf("migration failed: %w", err)
						}
					}
					
					// Register health checks if enabled
					if cfg.healthChecks && healthChecker != nil {
						// Health checks registration would be handled by the actual core registry
						// For now, we just log that the health checker is available
						logger.Info("Health checker initialized", zap.Int("databases", len(connections)))
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
									zap.String("database", name), 
									zap.Error(err))
							} else {
								logger.Info("Database connection closed", zap.String("database", name))
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
func newConnections(v *viper.Viper, logger *zap.Logger, cfg *moduleConfig) (Connections, error) {
	// Load configuration
	dbConfig, err := LoadConfig(v)
	if err != nil {
		return nil, fmt.Errorf("failed to load database configuration: %w", err)
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
func createConnection(name string, dbCfg *DatabaseConfig, logger *zap.Logger, cfg *moduleConfig) (*gorm.DB, error) {
	logger.Info("Creating database connection", zap.String("database", name), zap.String("driver", dbCfg.Driver))

	// Create GORM config
	gormCfg := &gorm.Config{
		SkipDefaultTransaction: dbCfg.SkipDefaultTx,
		PrepareStmt:           dbCfg.PrepareStmt,
		Logger:                NewGormLogger(logger, dbCfg.LogLevel, dbCfg.SlowThreshold),
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
		dialector = postgres.Open(dbCfg.DSN)
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

	logger.Info("Database connection created successfully", zap.String("database", name))
	return db, nil
}

// testConnection tests a database connection
func testConnection(ctx context.Context, name string, db *gorm.DB, logger *zap.Logger) error {
	logger.Info("Testing database connection", zap.String("database", name))
	
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

	logger.Info("Database connection test successful", zap.String("database", name))
	return nil
}