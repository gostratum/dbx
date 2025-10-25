# DBX Read Replicas Example

This example demonstrates how to configure and use read replicas with the DBX module for scalable database architecture.

## Features

- **Primary Database**: Handles all write operations (INSERT, UPDATE, DELETE)
- **Read Replicas**: Handle read operations (SELECT) with load balancing
- **Automatic Routing**: Writes go to primary, reads can use replicas
- **Explicit Control**: Force queries to use primary or replicas
- **Load Balancing**: Automatic distribution across multiple replicas

## Quick Start

### 1. Start PostgreSQL (Primary + 2 Replicas)

```bash
docker-compose up -d
```

This starts:
- **Primary**: localhost:5432
- **Replica 1**: localhost:5433
- **Replica 2**: localhost:5434

### 2. Run the Example

```bash
# Note: Use GOWORK=off to build with local go.mod instead of workspace
GOWORK=off go run main.go

# Or build the binary
GOWORK=off go build .
./read-replicas-example
```

## Configuration

See `configs/base.yaml`:

```yaml
db:
  default: primary
  databases:
    primary:
      driver: postgres
      dsn: postgres://postgres:postgres@localhost:5432/demo?sslmode=disable
      
      # Read replicas for load balancing
      read_replicas:
        - postgres://postgres:postgres@localhost:5433/demo?sslmode=disable
        - postgres://postgres:postgres@localhost:5434/demo?sslmode=disable
```

## Usage Patterns

### 1. Default Behavior (Automatic Routing)

```go
// Writes always go to primary
db.Create(&user)           // → Primary
db.Update(&user)           // → Primary
db.Delete(&user)           // → Primary

// Reads can use replicas
db.Find(&users)            // → Replica (load balanced)
db.First(&user, 1)         // → Replica (load balanced)
```

### 2. Force Primary Database

Use this after writes when you need strong consistency:

```go
// Create user (goes to primary)
db.Create(&user)

// Immediately read the same user from primary
// This ensures you get the latest data
db.Clauses(dbx.WithPrimary()).First(&user, user.ID)
```

### 3. Force Read Replicas

Explicitly use replicas for read-heavy operations:

```go
// Large read query - ensure it uses replicas
db.Clauses(dbx.WithReadReplicas()).
    Where("status = ?", "active").
    Find(&users)
```

### 4. Transactions (Always Primary)

Transactions automatically use the primary database:

```go
db.Transaction(func(tx *gorm.DB) error {
    // All operations in transaction use primary
    tx.Create(&user)
    tx.Create(&order)
    return nil
})
```

## Read Replica Features

### Load Balancing

DBX uses GORM's dbresolver plugin with random load balancing:
- Queries are distributed across all healthy replicas
- Failed replicas are automatically skipped
- No single point of failure for reads

### Connection Pooling

Each replica has its own connection pool:
- Configurable via `max_open_conns`, `max_idle_conns`
- Independent pool management
- Optimal resource utilization

### Health Checks

Health checks verify both primary and replicas:
- Automatic health monitoring
- Failed replicas excluded from load balancing
- Kubernetes readiness/liveness integration

## Production Deployment

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dbx-config
data:
  base.yaml: |
    db:
      default: primary
      databases:
        primary:
          dsn: postgres://user:pass@primary-svc:5432/db
          read_replicas:
            - postgres://user:pass@replica1-svc:5432/db
            - postgres://user:pass@replica2-svc:5432/db
```

### Environment Variables

```bash
# Override via environment variables
export STRATUM_DB_DATABASES_PRIMARY_DSN=postgres://...
export STRATUM_DB_DATABASES_PRIMARY_READ_REPLICAS_0=postgres://replica1...
export STRATUM_DB_DATABASES_PRIMARY_READ_REPLICAS_1=postgres://replica2...
```

## Replication Setup

**Note:** This example uses independent PostgreSQL instances for demonstration. In production, set up actual PostgreSQL replication:

### PostgreSQL Streaming Replication

```bash
# On primary
pg_basebackup -h primary -D /var/lib/postgresql/data -U replicator -P -v

# On replica
recovery.conf:
primary_conninfo = 'host=primary port=5432 user=replicator'
standby_mode = 'on'
```

### Cloud Provider Solutions

- **AWS RDS**: Multi-AZ with read replicas
- **Google Cloud SQL**: Read replicas across zones
- **Azure Database**: Read replicas in multiple regions

## Monitoring

Monitor replica lag and health:

```go
// Check replica status (example)
var lag int
db.Raw("SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())).
    Scan(&lag)
```

## Troubleshooting

### Replica Not Being Used

Check:
1. Read replicas configured correctly in YAML
2. DSNs are valid and accessible
3. No transaction active (transactions use primary)

### Replication Lag

If reads return stale data:
- Use `dbx.WithPrimary()` for critical reads
- Monitor replication lag
- Increase replica resources
- Consider read-after-write consistency patterns

## Best Practices

1. **Use replicas for reporting** - Offload heavy analytics to replicas
2. **Force primary after writes** - When strong consistency needed
3. **Monitor replica lag** - Set alerts for excessive lag
4. **Connection pooling** - Size pools based on load
5. **Failover strategy** - Plan for replica failures

## Clean Up

```bash
docker-compose down -v
```

## Learn More

- [GORM DBResolver Documentation](https://gorm.io/docs/dbresolver.html)
- [PostgreSQL Replication](https://www.postgresql.org/docs/current/high-availability.html)
- [Read Replica Patterns](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ReadRepl.html)
