# Simple DBX Example

A minimal example demonstrating the **new configuration and health check patterns** in the DBX module.

## What This Example Demonstrates

### ✨ Modern Patterns
1. **Configuration with `core/configx`** - New loader-based configuration
2. **Health Checks with `core.Registry`** - Unified health monitoring
3. **Dependency Injection** - Clean DI with `*gorm.DB`
4. **Structured Logging** - Using `core/logx`

## Quick Start

```bash
# Run the example
cd examples/simple-dbx
go run main.go
```

## Code Walkthrough

### 1. Configuration (config.yaml)

```yaml
# New dbx configuration format
db:
  default: primary      # Name of the default database
  databases:
    primary:
      driver: sqlite
      dsn: "file::memory:?cache=shared"
      max_open_conns: 10
      max_idle_conns: 5
      log_level: info
      slow_threshold: 100ms
```

**Key Points:**
- Uses `db.databases.<name>` structure (not `database.default`)
- Implements `configx.Configurable` interface
- Configuration prefix is `"db"`

### 2. Application Setup (main.go)

```go
package main

import (
    "github.com/gostratum/core"
    "github.com/gostratum/dbx"
    "go.uber.org/fx"
    "gorm.io/gorm"
)

type User struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string
    Email string `gorm:"uniqueIndex"`
}

func main() {
    app := core.New(
        // DBX module with health checks enabled
        dbx.Module(
            dbx.WithDefault("primary"),
            dbx.WithAutoMigrate(&User{}),
            dbx.WithHealthChecks(),
        ),
        
        // Your application logic
        fx.Invoke(RunDemo),
    )
    
    app.Run()
}

func RunDemo(db *gorm.DB, logger logx.Logger, lc fx.Lifecycle) {
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            // Create a user
            user := &User{Name: "John Doe", Email: "john@example.com"}
            if err := db.Create(user).Error; err != nil {
                return err
            }
            
            logger.Info("User created", 
                logx.Int("id", int(user.ID)),
                logx.String("email", user.Email))
            
            return nil
        },
    })
}
```

**Key Points:**
- Inject `*gorm.DB` directly (not `Connections`)
- Use `WithHealthChecks()` to enable health monitoring
- Clean, simple service constructors

### 3. Environment Variables

Override configuration using environment variables:

```bash
# Set database connection
export STRATUM_DB_DEFAULT=primary
export STRATUM_DB_DATABASES_PRIMARY_DSN="postgres://localhost/mydb"

# Run the app
go run main.go
```

**Key Points:**
- All environment variables use `STRATUM_` prefix
- Nested keys use underscores: `DB_DATABASES_PRIMARY_DSN`
- Automatically handled by `core/configx`

### 4. Health Checks

If you add the HTTP module, health endpoints are automatically available:

```go
app := core.New(
    httpx.Module(),  // Adds HTTP server with health endpoints
    dbx.Module(
        dbx.WithHealthChecks(),  // Registers DB health checks
    ),
)
```

Then check health:
```bash
# Readiness check
curl http://localhost:8080/health/ready

# Liveness check
curl http://localhost:8080/health/live
```

**Response:**
```json
{
  "ok": true,
  "details": {
    "db-primary-readiness": {"ok": true, "error": ""},
    "db-primary-liveness": {"ok": true, "error": ""}
  }
}
```

## Pattern Comparison

### Configuration Loading

**❌ Old Pattern:**
```go
// Using viper directly
cfg, err := dbx.LoadConfig(viper.GetViper())
```

**✅ New Pattern:**
```go
// Automatic via core/configx - no manual code needed
app := core.New(
    dbx.Module(),  // Configuration loaded automatically
)
```

### Database Injection

**❌ Old Pattern:**
```go
func NewService(conns dbx.Connections, logger logx.Logger) {
    db, exists := conns["default"]
    if !exists {
        panic("no default db")
    }
    // Use db...
}
```

**✅ New Pattern:**
```go
func NewService(db *gorm.DB, logger logx.Logger) {
    // Use db directly!
}
```

### Health Checks

**❌ Old Pattern:**
```go
// Custom HealthRegistry interface
type MyRegistry struct{}
func (r *MyRegistry) RegisterReadinessCheck(check *dbx.HealthCheck) error {...}
```

**✅ New Pattern:**
```go
// Automatic registration with core.Registry
app := core.New(
    dbx.Module(dbx.WithHealthChecks()),  // Done!
)
```

## Benefits

### 1. Consistency
- Same patterns across all modules (dbx, metricsx, tracingx)
- Unified configuration system
- Unified health check system

### 2. Simplicity
- Less boilerplate code
- Cleaner dependency injection
- Automatic configuration loading

### 3. Testability
- Easier to mock `*gorm.DB`
- Configuration loader can be mocked
- Health registry can be mocked

### 4. Production Ready
- Kubernetes-compatible health checks
- Proper lifecycle management
- Graceful shutdown

## Next Steps

1. **Add HTTP Server**: Include `httpx.Module()` for REST API
2. **Add Metrics**: Include `metricsx.Module()` for monitoring
3. **Add Tracing**: Include `tracingx.Module()` for distributed tracing
4. **Multiple Databases**: Configure additional databases in `config.yaml`

## See Also

- [Observability Demo](../observability-demo/) - Full example with metrics and tracing
- [Order Service](../orderservice/) - Production-ready example
- [DBX Documentation](../../dbx/README.md) - Complete module documentation
