package dbx

import (
	"context"
	"time"

	"github.com/gostratum/metricsx"
	"gorm.io/gorm"
)

const (
	metricsPluginName = "metricsx"
	startTimeKey      = "dbx:start_time"
)

// MetricsPlugin implements GORM plugin interface for metrics collection
type MetricsPlugin struct {
	metrics metricsx.Metrics

	// Metric collectors
	queryCounter  metricsx.Counter
	queryDuration metricsx.Histogram
	queryErrors   metricsx.Counter
	activeQueries metricsx.Gauge
	rowsAffected  metricsx.Histogram
}

// NewMetricsPlugin creates a new metrics plugin for GORM
func NewMetricsPlugin(metrics metricsx.Metrics) *MetricsPlugin {
	plugin := &MetricsPlugin{
		metrics: metrics,
	}

	// Initialize metric collectors
	plugin.queryCounter = metrics.Counter(
		"db_queries_total",
		metricsx.WithHelp("Total number of database queries"),
		metricsx.WithLabels("database", "table", "operation"),
	)

	plugin.queryDuration = metrics.Histogram(
		"db_query_duration_seconds",
		metricsx.WithHelp("Database query duration in seconds"),
		metricsx.WithLabels("database", "table", "operation"),
		metricsx.WithBuckets(0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0),
	)

	plugin.queryErrors = metrics.Counter(
		"db_query_errors_total",
		metricsx.WithHelp("Total number of database query errors"),
		metricsx.WithLabels("database", "table", "operation", "error_type"),
	)

	plugin.activeQueries = metrics.Gauge(
		"db_queries_in_flight",
		metricsx.WithHelp("Current number of database queries being executed"),
		metricsx.WithLabels("database"),
	)

	plugin.rowsAffected = metrics.Histogram(
		"db_rows_affected",
		metricsx.WithHelp("Number of rows affected by database operations"),
		metricsx.WithLabels("database", "table", "operation"),
		metricsx.WithBuckets(1, 10, 50, 100, 500, 1000, 5000, 10000),
	)

	return plugin
}

// Name returns the plugin name
func (p *MetricsPlugin) Name() string {
	return metricsPluginName
}

// Initialize implements gorm.Plugin interface
func (p *MetricsPlugin) Initialize(db *gorm.DB) error {
	// Register callbacks for tracking queries
	if err := p.registerCallbacks(db); err != nil {
		return err
	}

	return nil
}

// registerCallbacks registers GORM callbacks for metrics collection
func (p *MetricsPlugin) registerCallbacks(db *gorm.DB) error {
	// Get database name from config if available
	dbName := "default"
	if dbConfig, ok := db.Config.ConnPool.(interface{ Name() string }); ok {
		dbName = dbConfig.Name()
	}

	// Register before callbacks to track start time and active queries
	beforeCallbacks := []string{
		"gorm:create:before",
		"gorm:query:before",
		"gorm:update:before",
		"gorm:delete:before",
		"gorm:raw:before",
		"gorm:row:before",
	}

	for _, name := range beforeCallbacks {
		if err := db.Callback().Create().Before(name).Register(metricsPluginName+":before", p.before(dbName)); err != nil {
			return err
		}
		if err := db.Callback().Query().Before(name).Register(metricsPluginName+":before", p.before(dbName)); err != nil {
			return err
		}
		if err := db.Callback().Update().Before(name).Register(metricsPluginName+":before", p.before(dbName)); err != nil {
			return err
		}
		if err := db.Callback().Delete().Before(name).Register(metricsPluginName+":before", p.before(dbName)); err != nil {
			return err
		}
		if err := db.Callback().Raw().Before(name).Register(metricsPluginName+":before", p.before(dbName)); err != nil {
			return err
		}
		if err := db.Callback().Row().Before(name).Register(metricsPluginName+":before", p.before(dbName)); err != nil {
			return err
		}
	}

	// Register after callbacks to track metrics
	if err := db.Callback().Create().After("gorm:create:after").Register(metricsPluginName+":after", p.after(dbName, "create")); err != nil {
		return err
	}
	if err := db.Callback().Query().After("gorm:query:after").Register(metricsPluginName+":after", p.after(dbName, "select")); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update:after").Register(metricsPluginName+":after", p.after(dbName, "update")); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete:after").Register(metricsPluginName+":after", p.after(dbName, "delete")); err != nil {
		return err
	}
	if err := db.Callback().Raw().After("gorm:raw:after").Register(metricsPluginName+":after", p.after(dbName, "raw")); err != nil {
		return err
	}
	if err := db.Callback().Row().After("gorm:row:after").Register(metricsPluginName+":after", p.after(dbName, "row")); err != nil {
		return err
	}

	return nil
}

// before is called before query execution
func (p *MetricsPlugin) before(dbName string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		// Store start time in context
		db.InstanceSet(startTimeKey, time.Now())

		// Increment active queries
		p.activeQueries.Inc(dbName)
	}
}

// after is called after query execution
func (p *MetricsPlugin) after(dbName, operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		// Decrement active queries
		p.activeQueries.Dec(dbName)

		// Get table name
		tableName := db.Statement.Table
		if tableName == "" {
			tableName = "unknown"
		}

		// Calculate duration
		var duration float64
		if startTime, ok := db.InstanceGet(startTimeKey); ok {
			if t, ok := startTime.(time.Time); ok {
				duration = time.Since(t).Seconds()
			}
		}

		// Record metrics
		p.queryCounter.Inc(dbName, tableName, operation)
		p.queryDuration.Observe(duration, dbName, tableName, operation)

		// Record rows affected if available
		if db.RowsAffected > 0 {
			p.rowsAffected.Observe(float64(db.RowsAffected), dbName, tableName, operation)
		}

		// Record errors
		if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
			errorType := "unknown"
			switch db.Error {
			case context.Canceled:
				errorType = "canceled"
			case context.DeadlineExceeded:
				errorType = "timeout"
			default:
				errorType = "query_error"
			}
			p.queryErrors.Inc(dbName, tableName, operation, errorType)
		}
	}
}

// ConnectionPoolMetrics adds connection pool metrics if available
func ConnectionPoolMetrics(metrics metricsx.Metrics, db *gorm.DB, dbName string) {
	// Create connection pool metrics
	maxOpenConnections := metrics.Gauge(
		"db_max_open_connections",
		metricsx.WithHelp("Maximum number of open connections to the database"),
		metricsx.WithLabels("database"),
	)

	openConnections := metrics.Gauge(
		"db_open_connections",
		metricsx.WithHelp("Current number of open connections to the database"),
		metricsx.WithLabels("database"),
	)

	inUseConnections := metrics.Gauge(
		"db_in_use_connections",
		metricsx.WithHelp("Current number of connections in use"),
		metricsx.WithLabels("database"),
	)

	idleConnections := metrics.Gauge(
		"db_idle_connections",
		metricsx.WithHelp("Current number of idle connections"),
		metricsx.WithLabels("database"),
	)

	waitCount := metrics.Gauge(
		"db_wait_count_total",
		metricsx.WithHelp("Total number of connections waited for"),
		metricsx.WithLabels("database"),
	)

	waitDuration := metrics.Gauge(
		"db_wait_duration_seconds",
		metricsx.WithHelp("Total time spent waiting for connections"),
		metricsx.WithLabels("database"),
	)

	maxIdleClosed := metrics.Gauge(
		"db_max_idle_closed_total",
		metricsx.WithHelp("Total number of connections closed due to max idle"),
		metricsx.WithLabels("database"),
	)

	maxLifetimeClosed := metrics.Gauge(
		"db_max_lifetime_closed_total",
		metricsx.WithHelp("Total number of connections closed due to max lifetime"),
		metricsx.WithLabels("database"),
	)

	// Get SQL DB for stats
	sqlDB, err := db.DB()
	if err != nil {
		return
	}

	// Update metrics periodically
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			stats := sqlDB.Stats()

			maxOpenConnections.Set(float64(stats.MaxOpenConnections), dbName)
			openConnections.Set(float64(stats.OpenConnections), dbName)
			inUseConnections.Set(float64(stats.InUse), dbName)
			idleConnections.Set(float64(stats.Idle), dbName)
			waitCount.Set(float64(stats.WaitCount), dbName)
			waitDuration.Set(stats.WaitDuration.Seconds(), dbName)
			maxIdleClosed.Set(float64(stats.MaxIdleClosed), dbName)
			maxLifetimeClosed.Set(float64(stats.MaxLifetimeClosed), dbName)
		}
	}()
}
