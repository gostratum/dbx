package migrate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gostratum/metricsx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMetrics implements a simple in-memory metrics collector for testing
type mockMetrics struct {
	counters   map[string]float64
	histograms map[string][]float64
	gauges     map[string]float64
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{
		counters:   make(map[string]float64),
		histograms: make(map[string][]float64),
		gauges:     make(map[string]float64),
	}
}

func (m *mockMetrics) Counter(name string, opts ...metricsx.Option) metricsx.Counter {
	return &mockCounter{metrics: m, name: name}
}

func (m *mockMetrics) Histogram(name string, opts ...metricsx.Option) metricsx.Histogram {
	return &mockHistogram{metrics: m, name: name}
}

func (m *mockMetrics) Gauge(name string, opts ...metricsx.Option) metricsx.Gauge {
	return &mockGauge{metrics: m, name: name}
}

func (m *mockMetrics) Summary(name string, opts ...metricsx.Option) metricsx.Summary {
	return &mockSummary{metrics: m, name: name}
}

type mockCounter struct {
	metrics *mockMetrics
	name    string
}

func (c *mockCounter) Inc(labels ...string) {
	key := c.name + ":" + joinLabels(labels)
	c.metrics.counters[key]++
}

func (c *mockCounter) Add(value float64, labels ...string) {
	key := c.name + ":" + joinLabels(labels)
	c.metrics.counters[key] += value
}

type mockHistogram struct {
	metrics *mockMetrics
	name    string
}

func (h *mockHistogram) Observe(value float64, labels ...string) {
	key := h.name + ":" + joinLabels(labels)
	h.metrics.histograms[key] = append(h.metrics.histograms[key], value)
}

func (h *mockHistogram) Timer(labels ...string) metricsx.Timer {
	return &mockTimer{start: time.Now()}
}

type mockTimer struct {
	start time.Time
}

func (t *mockTimer) ObserveDuration() {
	// No-op for tests
}

func (t *mockTimer) Stop() time.Duration {
	return time.Since(t.start)
}

type mockGauge struct {
	metrics *mockMetrics
	name    string
}

func (g *mockGauge) Set(value float64, labels ...string) {
	key := g.name + ":" + joinLabels(labels)
	g.metrics.gauges[key] = value
}

func (g *mockGauge) Inc(labels ...string) {
	key := g.name + ":" + joinLabels(labels)
	g.metrics.gauges[key]++
}

func (g *mockGauge) Dec(labels ...string) {
	key := g.name + ":" + joinLabels(labels)
	g.metrics.gauges[key]--
}

func (g *mockGauge) Add(value float64, labels ...string) {
	key := g.name + ":" + joinLabels(labels)
	g.metrics.gauges[key] += value
}

func (g *mockGauge) Sub(value float64, labels ...string) {
	key := g.name + ":" + joinLabels(labels)
	g.metrics.gauges[key] -= value
}

type mockSummary struct {
	metrics *mockMetrics
	name    string
}

func (s *mockSummary) Observe(value float64, labels ...string) {
	// No-op for tests
}

func joinLabels(labels []string) string {
	result := ""
	for _, label := range labels {
		if result != "" {
			result += ","
		}
		result += label
	}
	return result
}

func TestNewMigrationMetrics(t *testing.T) {
	t.Run("creates_metrics_with_valid_provider", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		require.NotNil(t, migrationMetrics)
		assert.NotNil(t, migrationMetrics.metrics)
		assert.NotNil(t, migrationMetrics.operationsTotal)
		assert.NotNil(t, migrationMetrics.operationDuration)
		assert.NotNil(t, migrationMetrics.currentVersion)
		assert.NotNil(t, migrationMetrics.pendingCount)
		assert.NotNil(t, migrationMetrics.dirtyState)
	})

	t.Run("returns_nil_with_nil_provider", func(t *testing.T) {
		migrationMetrics := NewMigrationMetrics(nil)
		assert.Nil(t, migrationMetrics)
	})
}

func TestMigrationMetrics_RecordOperation(t *testing.T) {
	t.Run("records_successful_operation", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		migrationMetrics.RecordOperation("testdb", "up", 2*time.Second, nil)

		// Check counter was incremented
		counterKey := "db_migration_operations_total:testdb,up,success"
		assert.Equal(t, 1.0, mockMetrics.counters[counterKey])

		// Check histogram recorded duration
		histogramKey := "db_migration_duration_seconds:testdb,up"
		require.Len(t, mockMetrics.histograms[histogramKey], 1)
		assert.InDelta(t, 2.0, mockMetrics.histograms[histogramKey][0], 0.01)
	})

	t.Run("records_failed_operation", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		migrationMetrics.RecordOperation("testdb", "down", 1*time.Second, errors.New("migration failed"))

		// Check counter was incremented with failure status
		counterKey := "db_migration_operations_total:testdb,down,failure"
		assert.Equal(t, 1.0, mockMetrics.counters[counterKey])

		// Check histogram recorded duration
		histogramKey := "db_migration_duration_seconds:testdb,down"
		require.Len(t, mockMetrics.histograms[histogramKey], 1)
		assert.InDelta(t, 1.0, mockMetrics.histograms[histogramKey][0], 0.01)
	})

	t.Run("handles_nil_metrics", func(t *testing.T) {
		var migrationMetrics *MigrationMetrics
		// Should not panic
		migrationMetrics.RecordOperation("testdb", "up", 1*time.Second, nil)
	})
}

func TestMigrationMetrics_UpdateStatus(t *testing.T) {
	t.Run("updates_all_status_metrics", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		migrationMetrics.UpdateStatus("testdb", 42, false, 5)

		// Check current version
		assert.Equal(t, 42.0, mockMetrics.gauges["db_migration_version_current:testdb"])

		// Check pending count
		assert.Equal(t, 5.0, mockMetrics.gauges["db_migration_pending_count:testdb"])

		// Check dirty state (clean)
		assert.Equal(t, 0.0, mockMetrics.gauges["db_migration_dirty:testdb"])
	})

	t.Run("sets_dirty_state_to_1_when_dirty", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		migrationMetrics.UpdateStatus("testdb", 10, true, 0)

		// Check dirty state
		assert.Equal(t, 1.0, mockMetrics.gauges["db_migration_dirty:testdb"])
	})

	t.Run("handles_nil_metrics", func(t *testing.T) {
		var migrationMetrics *MigrationMetrics
		// Should not panic
		migrationMetrics.UpdateStatus("testdb", 1, false, 0)
	})
}

func TestMigrationMetrics_trackOperation(t *testing.T) {
	t.Run("tracks_successful_operation", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		called := false
		err := migrationMetrics.trackOperation("testdb", "test_op", func() error {
			called = true
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		assert.NoError(t, err)
		assert.True(t, called)

		// Check metrics were recorded
		counterKey := "db_migration_operations_total:testdb,test_op,success"
		assert.Equal(t, 1.0, mockMetrics.counters[counterKey])

		histogramKey := "db_migration_duration_seconds:testdb,test_op"
		require.Len(t, mockMetrics.histograms[histogramKey], 1)
		assert.Greater(t, mockMetrics.histograms[histogramKey][0], 0.0)
	})

	t.Run("tracks_failed_operation", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		testError := errors.New("test error")
		err := migrationMetrics.trackOperation("testdb", "test_op", func() error {
			return testError
		})

		assert.Equal(t, testError, err)

		// Check failure was recorded
		counterKey := "db_migration_operations_total:testdb,test_op,failure"
		assert.Equal(t, 1.0, mockMetrics.counters[counterKey])
	})

	t.Run("executes_function_with_nil_metrics", func(t *testing.T) {
		var migrationMetrics *MigrationMetrics

		called := false
		err := migrationMetrics.trackOperation("testdb", "test_op", func() error {
			called = true
			return nil
		})

		assert.NoError(t, err)
		assert.True(t, called)
	})
}

func TestExtractDatabaseFromURL(t *testing.T) {
	tests := []struct {
		name     string
		dbURL    string
		expected string
	}{
		{
			name:     "returns_default_for_any_url",
			dbURL:    "postgres://user:pass@localhost:5432/mydb",
			expected: "default",
		},
		{
			name:     "returns_default_for_empty_url",
			dbURL:    "",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDatabaseFromURL(tt.dbURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithMetrics(t *testing.T) {
	t.Run("calls_function_and_tracks_metrics", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		called := false
		callCount := 0
		fn := func(ctx context.Context, dbURL string) error {
			called = true
			callCount++
			assert.Equal(t, "test://db", dbURL)
			return nil
		}

		ctx := context.Background()
		err := WithMetrics(ctx, "test://db", migrationMetrics, "test_operation", fn)

		assert.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, 1, callCount)

		// Check metrics
		counterKey := "db_migration_operations_total:default,test_operation,success"
		assert.Equal(t, 1.0, mockMetrics.counters[counterKey])
	})

	t.Run("calls_function_without_metrics", func(t *testing.T) {
		called := false
		fn := func(ctx context.Context, dbURL string) error {
			called = true
			return nil
		}

		ctx := context.Background()
		err := WithMetrics(ctx, "test://db", nil, "test_operation", fn)

		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("propagates_errors", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		testError := errors.New("test error")
		fn := func(ctx context.Context, dbURL string) error {
			return testError
		}

		ctx := context.Background()
		err := WithMetrics(ctx, "test://db", migrationMetrics, "test_operation", fn)

		assert.Equal(t, testError, err)

		// Check failure was recorded
		counterKey := "db_migration_operations_total:default,test_operation,failure"
		assert.Equal(t, 1.0, mockMetrics.counters[counterKey])
	})
}

func TestMigrationMetrics_MultipleOperations(t *testing.T) {
	t.Run("tracks_multiple_operations", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		// Record multiple successful operations
		migrationMetrics.RecordOperation("db1", "up", 1*time.Second, nil)
		migrationMetrics.RecordOperation("db1", "up", 2*time.Second, nil)
		migrationMetrics.RecordOperation("db1", "down", 500*time.Millisecond, nil)

		// Record failed operation
		migrationMetrics.RecordOperation("db2", "up", 3*time.Second, errors.New("failed"))

		// Update status for different databases
		migrationMetrics.UpdateStatus("db1", 5, false, 2)
		migrationMetrics.UpdateStatus("db2", 3, true, 10)

		// Verify counters
		assert.Equal(t, 2.0, mockMetrics.counters["db_migration_operations_total:db1,up,success"])
		assert.Equal(t, 1.0, mockMetrics.counters["db_migration_operations_total:db1,down,success"])
		assert.Equal(t, 1.0, mockMetrics.counters["db_migration_operations_total:db2,up,failure"])

		// Verify histograms
		assert.Len(t, mockMetrics.histograms["db_migration_duration_seconds:db1,up"], 2)
		assert.Len(t, mockMetrics.histograms["db_migration_duration_seconds:db1,down"], 1)

		// Verify gauges
		assert.Equal(t, 5.0, mockMetrics.gauges["db_migration_version_current:db1"])
		assert.Equal(t, 2.0, mockMetrics.gauges["db_migration_pending_count:db1"])
		assert.Equal(t, 0.0, mockMetrics.gauges["db_migration_dirty:db1"])

		assert.Equal(t, 3.0, mockMetrics.gauges["db_migration_version_current:db2"])
		assert.Equal(t, 10.0, mockMetrics.gauges["db_migration_pending_count:db2"])
		assert.Equal(t, 1.0, mockMetrics.gauges["db_migration_dirty:db2"])
	})
}

func TestMigrationMetrics_ConcurrentOperations(t *testing.T) {
	t.Run("handles_concurrent_metric_updates", func(t *testing.T) {
		mockMetrics := newMockMetrics()
		migrationMetrics := NewMigrationMetrics(mockMetrics)

		// This test ensures no panics occur with concurrent access
		// In a real implementation with actual metrics libraries,
		// they handle concurrency internally

		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				migrationMetrics.RecordOperation("testdb", "up", time.Millisecond*100, nil)
				migrationMetrics.UpdateStatus("testdb", uint(id), false, id)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Just verify something was recorded
		counterKey := "db_migration_operations_total:testdb,up,success"
		assert.Greater(t, mockMetrics.counters[counterKey], 0.0)
	})
}
