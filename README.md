# 🗄️ DBX - Database Module for Gostratum

A thin, composable SQL layer built on top of [**gostratum/core**](https://github.com/gostratum/core) providing **GORM** database integration with **Fx-first composition**, **core/configx configuration**, and **core health check integration**.

## 📦 Features

- **Fx-first composition** - No globals, pure dependency injection
- **Multi-database support** - Manage multiple database connections
- **Read replica support** - Automatic read/write splitting for scalability
- **Config-driven setup** - Uses `core/configx` for configuration with sensible defaults
- **Auto-migration support** - GORM model auto-migration and SQL file migrations
- **Health integration** - Automatic readiness/liveness checks via `core.Registry`
- **Lifecycle management** - Proper connection handling and graceful shutdown
- **Observability-ready** - Uses `core/logx` for structured logging
- **Transaction helpers** - Simplified transaction management
- **Connection pooling** - Configurable connection pool settings

## 🚀 Quick Start

### Installation

```bash
go get github.com/gostratum/dbx
```

### Basic Usage

```go
package main

import (
    "github.com/gostratum/core"
    "github.com/gostratum/dbx"
    "go.uber.org/fx"
    "gorm.io/gorm"
)

type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string
}

func main() {
    app := core.New(
        dbx.Module(
            dbx.WithDefault("primary"),
            dbx.WithAutoMigrate(&User{}),
        ),
        fx.Invoke(func(db *gorm.DB) {
            // Use your database connection
            var users []User
            db.Find(&users)
        }),
    )
    app.Run()
}
```

### With HTTP Server

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/gostratum/core"
    "github.com/gostratum/dbx"
    "github.com/gostratum/httpx"
    "go.uber.org/fx"
    "gorm.io/gorm"
)

func main() {
    app := core.New(
        dbx.Module(
            dbx.WithDefault("primary"),
            dbx.WithAutoMigrate(&User{}),
            dbx.WithHealthChecks(),
        ),
        httpx.Module(httpx.WithBasePath("/api")),
        fx.Invoke(func(e *gin.Engine, db *gorm.DB) {
            e.GET("/api/users", func(c *gin.Context) {
                var users []User
                db.Find(&users)
                c.JSON(200, users)
            })
        }),
    )
    app.Run()
}
```

## ⚙️ Configuration

### Configuration Loading

The module uses `core/configx` for configuration loading. Configuration is automatically loaded from:

1. **Config files** in the `./configs` directory (or path specified by `CONFIG_PATHS` env var)
2. **Environment-specific files** (e.g., `dev.yaml`, `staging.yaml`, `prod.yaml` based on `APP_ENV`)
3. **Environment variables** with `STRATUM_` prefix

The configuration implements the `configx.Configurable` interface with the prefix `"db"`.

### Default Configuration

The module uses the following default configuration:

```yaml
db:
  default: primary
  databases:
    primary:
      driver: postgres
      dsn: "postgres://localhost/dbname?sslmode=disable"
      max_open_conns: 25
      max_idle_conns: 5
      conn_max_lifetime: 5m
      conn_max_idle_time: 5m
      log_level: warn
      slow_threshold: 200ms
      skip_default_tx: false
      prepare_stmt: true
```

### Environment Variables

Configuration can be overridden using environment variables with the `STRATUM_` prefix:

```bash
# Example environment variables
STRATUM_DB_DEFAULT=primary
STRATUM_DB_DATABASES_PRIMARY_DRIVER=postgres
STRATUM_DB_DATABASES_PRIMARY_DSN="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
STRATUM_DB_DATABASES_PRIMARY_MAX_OPEN_CONNS=50
STRATUM_DB_DATABASES_PRIMARY_MAX_IDLE_CONNS=10
STRATUM_DB_DATABASES_PRIMARY_LOG_LEVEL=info
```

Note: The `STRATUM_` prefix is added by the `core/configx` loader. Nested keys use underscores (`_`) as separators.

### Multiple Databases

```yaml
db:
  default: primary
  databases:
    primary:
      driver: postgres
      dsn: "postgres://localhost:5432/app_db?sslmode=disable"
      max_open_conns: 25
    analytics:
      driver: postgres
      dsn: "postgres://localhost:5432/analytics_db?sslmode=disable"
      max_open_conns: 10
      log_level: silent
```

```go
app := core.New(
    dbx.Module(dbx.WithDefault("primary")),
    fx.Invoke(func(provider *dbx.Provider) {
        primaryDB := provider.Get() // Gets default database
        analyticsDB := provider.GetByName("analytics")
        
        // Use different databases for different purposes
    }),
)
```

### Read Replicas (NEW ✨)

Configure read replicas for automatic read/write splitting:

```yaml
db:
  default: primary
  databases:
    primary:
      driver: postgres
      dsn: postgres://user:pass@primary:5432/db?sslmode=disable
      
      # Read replicas for load balancing
      read_replicas:
        - postgres://user:pass@replica1:5432/db?sslmode=disable
        - postgres://user:pass@replica2:5432/db?sslmode=disable
      
      max_open_conns: 25
```

**Usage:**

```go
// Writes automatically go to primary
db.Create(&user)           // → Primary
db.Update(&user)           // → Primary
db.Delete(&user)           // → Primary

// Reads can use replicas (automatic load balancing)
db.Find(&users)            // → Replica (round-robin)
db.First(&user, 1)         // → Replica

// Force primary database (for strong consistency after writes)
dbx.WithPrimary(db).First(&user, id)  // → Primary

// Explicitly use replicas
dbx.WithReadReplicas(db).Find(&users) // → Replica
```

**See:** [Read Replicas Example](examples/read-replicas/README.md) for complete setup with Docker Compose.

### Configuration Options

| Option | Description |
|--------|-------------|
| `driver` | Database driver (currently supports: `postgres`) |
| `dsn` | Database connection string |
| `max_open_conns` | Maximum number of open connections |
| `max_idle_conns` | Maximum number of idle connections |
| `conn_max_lifetime` | Maximum lifetime of a connection |
| `conn_max_idle_time` | Maximum idle time of a connection |
| `log_level` | GORM log level (`silent`, `error`, `warn`, `info`) |
| `slow_threshold` | Threshold for slow query logging |
| `skip_default_tx` | Skip default transactions for performance |
| `prepare_stmt` | Enable prepared statements |

## 🔧 Module Options

### `WithDefault(name string)`
Sets the default database connection name.

```go
dbx.Module(dbx.WithDefault("primary"))
```

### `WithAutoMigrate(models ...any)`
Enables GORM auto-migration for specified models.

```go
dbx.Module(dbx.WithAutoMigrate(&User{}, &Product{}, &Order{}))
```

### `WithMigrationsFS(fs fs.FS, dir ...string)`
Sets embedded filesystem for SQL migrations.

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

dbx.Module(
    dbx.WithMigrationsFS(migrationsFS, "migrations"),
    dbx.WithRunMigrations(),
)
```

### `WithRunMigrations()`
Enables running migrations at application startup.

```go
dbx.Module(
    dbx.WithAutoMigrate(&User{}),
    dbx.WithRunMigrations(),
)
```

### `WithGormConfig(cfg *gorm.Config)`
Provides custom GORM configuration.

```go
dbx.Module(
    dbx.WithGormConfig(&gorm.Config{
        SkipDefaultTransaction: true,
        PrepareStmt: true,
    }),
)
```

### `WithHealthChecks()`
Enables health check registration (enabled by default).

```go
dbx.Module(dbx.WithHealthChecks())
```

## 🏥 Health Checks

The module automatically registers readiness and liveness health checks for all configured databases:

- **Readiness checks**: Simple database ping (3-second timeout)
- **Liveness checks**: Database ping + connection pool validation + test query (5-second timeout)

Health checks are registered with the `core.Registry` and can be accessed via standard health endpoints when using `httpx` module.

### Kubernetes Probes

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:latest
    readinessProbe:
      httpGet:
        path: /health/ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
    livenessProbe:
      httpGet:
        path: /health/live
        port: 8080
      initialDelaySeconds: 15
      periodSeconds: 20
```

## 🔄 Migrations

### Auto-Migration

GORM models are automatically migrated when the application starts:

```go
type User struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string    `gorm:"not null"`
    Email     string    `gorm:"uniqueIndex"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

dbx.Module(
    dbx.WithAutoMigrate(&User{}),
    dbx.WithRunMigrations(),
)
```

### SQL Migrations

Embed SQL migration files and run them at startup:

```
migrations/
├── 001_create_indexes.sql
├── 002_add_constraints.sql
└── 003_seed_data.sql
```

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

dbx.Module(
    dbx.WithMigrationsFS(migrationsFS, "migrations"),
    dbx.WithRunMigrations(),
)
```

Migration files are executed in alphabetical order and tracked to prevent re-execution.

## 💾 Transaction Management

### Simple Transactions

```go
func transferMoney(db *gorm.DB, fromID, toID uint, amount float64) error {
    return dbx.WithTx(db, func(tx *gorm.DB) error {
        // Debit from account
        if err := tx.Model(&Account{}).Where("id = ?", fromID).
            Update("balance", gorm.Expr("balance - ?", amount)).Error; err != nil {
            return err
        }
        
        // Credit to account
        return tx.Model(&Account{}).Where("id = ?", toID).
            Update("balance", gorm.Expr("balance + ?", amount)).Error
    })
}
```

### Context-aware Transactions

```go
func processOrder(ctx context.Context, db *gorm.DB, order *Order) error {
    return dbx.WithTxContext(ctx, db, func(ctx context.Context, tx *gorm.DB) error {
        // Create order
        if err := tx.WithContext(ctx).Create(order).Error; err != nil {
            return err
        }
        
        // Update inventory
        return tx.WithContext(ctx).Model(&Product{}).
            Where("id = ?", order.ProductID).
            Update("stock", gorm.Expr("stock - ?", order.Quantity)).Error
    })
}
```

### Transaction Manager

```go
type OrderService struct {
    txManager *dbx.TxManager
}

func (s *OrderService) ProcessOrder(order *Order) error {
    return s.txManager.WithTx(func(tx *gorm.DB) error {
        // Business logic with savepoints
        if err := s.txManager.SavePoint(tx, "before_inventory"); err != nil {
            return err
        }
        
        // Update inventory
        if err := s.updateInventory(tx, order); err != nil {
            // Rollback to savepoint on error
            s.txManager.RollbackTo(tx, "before_inventory")
            return err
        }
        
        return nil
    })
}
```

## 🔍 Observability

### Logging Integration

The module integrates with Zap logger and provides structured logging for all database operations:

```go
// Automatic logging of:
// - SQL queries and execution time
// - Slow queries (configurable threshold)
// - Connection pool statistics
// - Transaction lifecycle
// - Migration progress

// Example log output:
{
  "level": "info",
  "time": "2023-01-01T12:00:00Z",
  "caller": "dbx/logger.go:45",
  "msg": "SQL query executed",
  "elapsed": "15ms",
  "sql": "SELECT * FROM users WHERE id = $1",
  "rows": 1,
  "trace_id": "abc123"
}
```

### Connection Pool Monitoring

```go
fx.Invoke(func(provider *dbx.Provider) {
    stats, err := provider.GetConnectionStats()
    if err != nil {
        return
    }
    
    for name, stat := range stats {
        log.Printf("Database %s: %d/%d connections in use", 
            name, stat.InUse, stat.MaxOpenConnections)
    }
})
```

## 📋 Examples

### Web Application

See [example/main.go](example/main.go) for a complete web application example with:
- Multiple models with auto-migration
- SQL file migrations
- REST API endpoints
- Health checks
- Transaction usage

### Worker Application

```go
package main

import (
    "context"
    "time"
    
    "github.com/gostratum/core"
    "github.com/gostratum/dbx"
    "go.uber.org/fx"
    "gorm.io/gorm"
)

type Job struct {
    ID        uint `gorm:"primaryKey"`
    Status    string
    ProcessedAt *time.Time
}

func main() {
    app := core.New(
        dbx.Module(
            dbx.WithDefault("primary"),
            dbx.WithAutoMigrate(&Job{}),
        ),
        fx.Invoke(func(lc fx.Lifecycle, db *gorm.DB) {
            lc.Append(fx.Hook{
                OnStart: func(ctx context.Context) error {
                    go processJobs(ctx, db)
                    return nil
                },
            })
        }),
    )
    app.Run()
}

func processJobs(ctx context.Context, db *gorm.DB) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            var jobs []Job
            db.Where("status = ?", "pending").Find(&jobs)
            
            for _, job := range jobs {
                processJob(ctx, db, &job)
            }
        }
    }
}
```

### Migration Job

```go
package main

import (
    "embed"
    "log"
    
    "github.com/gostratum/core"
    "github.com/gostratum/dbx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
    app := core.New(
        dbx.Module(
            dbx.WithMigrationsFS(migrationsFS, "migrations"),
            dbx.WithRunMigrations(),
        ),
    )
    
    // Run migrations and exit
    app.Run()
    log.Println("Migrations completed successfully")
}
```

## 🧪 Testing

### Config Loading Test

```go
func TestConfigLoading(t *testing.T) {
    // Create a test loader
    loader, err := newConfigLoader(`
db:
  default: test
  databases:
    test:
      driver: postgres
      dsn: "postgres://localhost/test"
`)
    require.NoError(t, err)
    
    cfg := &dbx.Config{}
    err = loader.Bind(cfg)
    assert.NoError(t, err)
    assert.Equal(t, "test", cfg.Default)
    assert.Equal(t, "postgres", cfg.Databases["test"].Driver)
}

```

### Health Check Test

```go
func TestHealthCheck(t *testing.T) {
    // Mock database setup
    db, mock := gormtest.NewMockDB()
    connections := dbx.Connections{"test": db}
    
    registry := core.NewHealthRegistry()
    checker := dbx.NewHealthChecker(connections, registry)
    
    mock.ExpectPing()
    
    err := checker.RegisterHealthChecks()
    assert.NoError(t, err)
    
    // Verify checks are registered
    result := registry.Aggregate(context.Background(), core.Readiness)
    assert.Contains(t, result.Details, "db-test-readiness")
}
```

## 🔧 Development

### Prerequisites

- Go 1.25+
- PostgreSQL (for testing)

### Running Tests

```bash
go test ./...
```

### Running Example

```bash
cd example
go run main.go
```

The example server will start on `:8080` with the following endpoints:
- `GET /api/health` - Health check
- `GET /api/users` - List users
- `POST /api/users` - Create user
- `GET /api/products` - List products
- `POST /api/products` - Create product

## 🚀 Database Migrations (golang-migrate)

The `dbx` module now includes first-class support for database migrations using [golang-migrate](https://github.com/golang-migrate/migrate). This provides production-grade migration capabilities with support for both embedded and filesystem migrations.

### Features

- ✅ **First-class migration support** using golang-migrate library
- ✅ **Embedded migrations** via `//go:embed` for easy deployment
- ✅ **Filesystem migrations** for development flexibility
- ✅ **CLI tool** for manual migration management
- ✅ **Fx integration** for automatic migrations on startup
- ✅ **Safe defaults** - AutoMigrate disabled by default for production safety
- ✅ **Postgres native** - Uses pgx driver for optimal performance

### Quick Start

#### 1. Using Embedded Migrations (Recommended for Production)

```go
package main

import (
    "github.com/gostratum/core"
    "github.com/gostratum/dbx"
    "go.uber.org/fx"
)

func main() {
    app := core.New(
        dbx.Module(
            dbx.WithGolangMigrateEmbed(), // Use embedded migrations from dbx/migrate/files
        ),
        fx.Invoke(func(db *gorm.DB) {
            // Your database is ready with all migrations applied!
        }),
    )
    app.Run()
}
```

#### 2. Using Filesystem Migrations (Development)

```go
app := core.New(
    dbx.Module(
        dbx.WithGolangMigrateDir("./migrations"),
    ),
)
```

### � New Unified Configuration (Recommended)

**Migration settings are now integrated directly into database configuration!** This makes it much more intuitive - migration settings live alongside database settings.

#### YAML Configuration (Production)

```yaml
db:
  default: primary
  databases:
    primary:
      # Database Connection
      driver: postgres
      dsn: postgres://user:pass@localhost:5432/mydb?sslmode=disable
      max_open_conns: 25
      log_level: info
      
      # Migration Settings (integrated!)
      migration_source: "embed://"              # Use embedded migrations
      auto_migrate: false                       # Safe default - NEVER enable in production!
      migration_table: "schema_migrations"      # Table for tracking migrations
      migration_lock_timeout: "15s"            # Lock timeout
      migration_verbose: false                  # Quiet logging
```

#### Your Application Code

```go
package main

import (
    "embed"
    "github.com/gostratum/core" 
    "github.com/gostratum/dbx"
    "go.uber.org/fx"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
    app := core.New(
        // Provide embedded migrations
        fx.Provide(func() embed.FS { return migrationsFS }),
        
        // DBX module - no migration options needed!
        // Everything configured via unified database config
        dbx.Module(),
        
        fx.Invoke(func(db *gorm.DB) {
            // Database ready with migrations applied!
        }),
    )
    app.Run()
}
```

#### Environment Variables

```bash
# Database connection
DB_DATABASES_PRIMARY_DRIVER=postgres
DB_DATABASES_PRIMARY_DSN=postgres://user:pass@localhost:5432/mydb

# Migration settings (unified under database config)
DB_DATABASES_PRIMARY_MIGRATION_SOURCE=embed://
DB_DATABASES_PRIMARY_AUTO_MIGRATE=false
DB_DATABASES_PRIMARY_MIGRATION_VERBOSE=true
```

#### Migration Source Options

| Format | Description | Use Case |
|--------|-------------|----------|
| `"embed://"` | Use embedded files (`//go:embed`) | **Production** - files in binary |
| `"file://./migrations"` | Read from filesystem | **Development** - easy iteration |
| `""` (empty) | No migrations | **Cache/read-only databases** |

### 📂 Where Do Migration Files Go?

Migration SQL files must be in **YOUR application's directory**:

```
your-app/
├── main.go              ← Contains: //go:embed migrations/*.sql
├── migrations/          ← YOUR SQL FILES HERE
│   ├── 000001_init.up.sql
│   ├── 000001_init.down.sql
│   └── 000002_add_orders.up.sql
└── go.mod
```





```bash
# Migration configuration using unified database environment variables
DB_DATABASES_PRIMARY_AUTO_MIGRATE=false
DB_DATABASES_PRIMARY_MIGRATION_SOURCE=embed://
DB_DATABASES_PRIMARY_MIGRATION_TABLE=schema_migrations
DB_DATABASES_PRIMARY_MIGRATION_LOCK_TIMEOUT=15s
DB_DATABASES_PRIMARY_MIGRATION_VERBOSE=true


### Creating Migrations

Migration files follow the naming convention: `{version}_{description}.{up|down}.sql`

#### Example Migration Files

**000001_init.up.sql**
```sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users(email);
```

**000001_init.down.sql**
```sql
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
```

#### Using the Makefile

```bash
# Create a new migration
make migrate-create NAME=add_orders_table

# This creates:
# - migrate/files/000002_add_orders_table.up.sql
# - migrate/files/000002_add_orders_table.down.sql
```

### Library API

Use the migration library programmatically:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/gostratum/dbx/migrate"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    dbURL := "postgres://user:pass@localhost:5432/dbname?sslmode=disable"

    // Apply all pending migrations
    if err := migrate.Up(ctx, dbURL, migrate.WithEmbed()); err != nil {
        if migrate.IsNoChange(err) {
            log.Println("No migrations to apply")
        } else {
            log.Fatal(err)
        }
    }

    // Get migration status
    status, err := migrate.GetStatus(ctx, dbURL, migrate.WithEmbed())
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Current version: %d, Dirty: %v", status.Current, status.Dirty)

    // Migrate to specific version
    if err := migrate.To(ctx, dbURL, 2, migrate.WithEmbed()); err != nil {
        log.Fatal(err)
    }

    // Revert migrations
    if err := migrate.Down(ctx, dbURL, migrate.WithEmbed()); err != nil {
        log.Fatal(err)
    }
}
```

### Production Best Practices

#### ⚠️ Safety Guidelines

1. **Never enable AutoMigrate in production**
   ```yaml
   # ❌ BAD - Automatic migrations in production
   dbx:
     migrate:
       auto_migrate: true
   
   # ✅ GOOD - Manual migrations only
   dbx:
     migrate:
       auto_migrate: false
   ```

2. **Use a migration review process**
   - All migrations should be code-reviewed
   - Test migrations in staging first
   - Have rollback plans ready

3. **Run migrations before deployment**
   ```go
   // In CI/CD pipeline, before deploying new code
   if err := migrate.Up(ctx, dbURL, migrate.WithEmbed()); err != nil {
       // Handle migration errors
   }
   ```

4. **Monitor migration status**
   ```go
   // Check migration status
   status, err := migrate.GetStatus(ctx, dbURL, migrate.WithEmbed())
   if err != nil {
       // Handle error
   }
   log.Printf("Version: %d, Dirty: %v", status.Current, status.Dirty)
   ```

5. **Handle migration failures gracefully**
   - Migrations are transactional (when possible)
   - Use `force` command only for recovery
   - Always backup before major migrations

#### CI/CD Integration

**GitHub Actions Example:**

```yaml
name: Database Migrations

on:
  push:
    branches: [main]

jobs:
  migrate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run migrations
        env:
          DBX_URL: ${{ secrets.DATABASE_URL }}
        run: |
          go run main.go -migrate # or use your application's migration command
```

**Kubernetes Init Container:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      initContainers:
      - name: migration
        image: myapp:latest
        command: ["/app/myapp", "-migrate"]  # Use your app's migration command
        env:
        - name: DB_DATABASES_PRIMARY_URL
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: url
      containers:
      - name: app
        image: myapp:latest
```

### Migration File Organization

```
dbx/
├── migrate/
│   ├── files/               # Embedded migration files
│   │   ├── 000001_init.up.sql
│   │   ├── 000001_init.down.sql
│   │   ├── 000002_add_orders.up.sql
│   │   └── 000002_add_orders.down.sql
│   ├── config.go            # Migration configuration
│   ├── migrate.go           # Public API
│   ├── errors.go            # Error types
│   └── internal/
│       ├── runner.go        # Migration runner
│       └── status.go        # Status tracking
```

### Troubleshooting

#### Dirty Migration State

If a migration fails partway through, the state becomes "dirty":

```go
// Check status
status, err := migrate.GetStatus(ctx, dbURL, migrate.WithEmbed())
if err != nil {
    log.Fatal(err)
}
log.Printf("Current Version: %d, Dirty: %v", status.Current, status.Dirty)

// Fix the issue in your migration file, then force the version
if err := migrate.Force(ctx, dbURL, 1, migrate.WithEmbed()); err != nil {
    log.Fatal(err)
}

// Re-run the migration
if err := migrate.Up(ctx, dbURL, migrate.WithEmbed()); err != nil {
    log.Fatal(err)
}
```

#### Lock Timeout

If migrations are locked (another process is running migrations):

```yaml
# Increase lock timeout
dbx:
  migrate:
    lock_timeout: "30s"  # Default is 15s
```

#### Migration Not Found

Ensure your migration files are properly embedded:

```go
// In dbx/migrate/source_embed.go
//go:embed files/*.sql
var EmbeddedMigrations embed.FS
```

### Comparison: GORM Auto-Migrate vs golang-migrate

| Feature | GORM Auto-Migrate | golang-migrate |
|---------|-------------------|----------------|
| **Control** | Automatic schema sync | Explicit SQL migrations |
| **Production Ready** | ⚠️ Risky | ✅ Safe |
| **Version Control** | No | Yes |
| **Rollback Support** | No | Yes |
| **Complex Changes** | Limited | Full SQL power |
| **Best For** | Development, simple schemas | Production, complex schemas |

**Recommendation:** Use golang-migrate for production applications and complex schema changes. GORM auto-migrate can be used for rapid prototyping.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built on top of [GORM](https://gorm.io/) - The fantastic Go ORM
- Inspired by the [Gostratum](https://github.com/gostratum) framework philosophy
- Uses [Uber Fx](https://uber-go.github.io/fx/) for dependency injection

---

> **Gostratum Philosophy**: "Thin, composable building blocks for modern Go cloud applications — each module adds one capability cleanly layered on `core`."