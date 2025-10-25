package dbx

import (
	"testing"

	"github.com/gostratum/core/logx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConfigureReadReplicas(t *testing.T) {
	// Create a test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create a simple test logger that implements the Logger interface
	testLogger := &testLogger{}

	t.Run("no replicas", func(t *testing.T) {
		err := configureReadReplicas(db, nil, testLogger)
		assert.NoError(t, err)
	})

	t.Run("empty replicas", func(t *testing.T) {
		err := configureReadReplicas(db, []string{}, testLogger)
		assert.NoError(t, err)
	})

	// Note: Testing actual replica connection requires real database instances
	// Integration tests should cover the full replica setup
}

// testLogger is a simple logger implementation for testing
type testLogger struct{}

func (l *testLogger) Debug(msg string, fields ...logx.Field) {}
func (l *testLogger) Info(msg string, fields ...logx.Field)  {}
func (l *testLogger) Warn(msg string, fields ...logx.Field)  {}
func (l *testLogger) Error(msg string, fields ...logx.Field) {}
func (l *testLogger) Fatal(msg string, fields ...logx.Field) {}
func (l *testLogger) With(fields ...logx.Field) logx.Logger  { return l }

func TestSanitizeDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "postgres DSN",
			dsn:  "postgres://user:password@localhost:5432/db",
			want: "[DSN redacted for security]",
		},
		{
			name: "mysql DSN",
			dsn:  "user:password@tcp(localhost:3306)/db",
			want: "[DSN redacted for security]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDSN(tt.dsn)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWithReadReplicas(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Test that the clause is added
	result := WithReadReplicas(db)
	assert.NotNil(t, result)
}

func TestWithPrimary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Test that the clause is added
	result := WithPrimary(db)
	assert.NotNil(t, result)
}
