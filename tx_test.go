package dbx

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}
	return db
}

func TestWithTx_Success(t *testing.T) {
	db := setupTestDB(t)
	
	// Create a test table
	err := db.Exec("CREATE TABLE test_users (id INTEGER PRIMARY KEY, name TEXT)").Error
	assert.NoError(t, err)
	
	// Test successful transaction
	err = WithTx(db, func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO test_users (name) VALUES (?)", "John").Error
	})
	
	assert.NoError(t, err)
	
	// Verify data was committed
	var count int64
	db.Table("test_users").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestWithTx_Rollback(t *testing.T) {
	db := setupTestDB(t)
	
	// Create a test table
	err := db.Exec("CREATE TABLE test_users (id INTEGER PRIMARY KEY, name TEXT)").Error
	assert.NoError(t, err)
	
	// Test transaction rollback
	testError := errors.New("test error")
	err = WithTx(db, func(tx *gorm.DB) error {
		tx.Exec("INSERT INTO test_users (name) VALUES (?)", "John")
		return testError // This should cause a rollback
	})
	
	assert.Equal(t, testError, err)
	
	// Verify data was rolled back
	var count int64
	db.Table("test_users").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestWithTxContext_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	
	// Create a test table
	err := db.Exec("CREATE TABLE test_users (id INTEGER PRIMARY KEY, name TEXT)").Error
	assert.NoError(t, err)
	
	// Test successful transaction with context
	err = WithTxContext(ctx, db, func(ctx context.Context, tx *gorm.DB) error {
		return tx.WithContext(ctx).Exec("INSERT INTO test_users (name) VALUES (?)", "John").Error
	})
	
	assert.NoError(t, err)
	
	// Verify data was committed
	var count int64
	db.Table("test_users").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestTxManager(t *testing.T) {
	db := setupTestDB(t)
	manager := NewTxManager(db)
	
	// Create a test table
	err := db.Exec("CREATE TABLE test_users (id INTEGER PRIMARY KEY, name TEXT)").Error
	assert.NoError(t, err)
	
	// Test transaction manager
	err = manager.WithTx(func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO test_users (name) VALUES (?)", "Alice").Error
	})
	
	assert.NoError(t, err)
	
	// Verify data was committed
	var count int64
	db.Table("test_users").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestTxWrapper(t *testing.T) {
	db := setupTestDB(t)
	wrapper := NewTxWrapper(db)
	
	// Create a test table
	err := db.Exec("CREATE TABLE test_users (id INTEGER PRIMARY KEY, name TEXT)").Error
	assert.NoError(t, err)
	
	// Test transaction wrapper
	err = wrapper.WithTx(func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO test_users (name) VALUES (?)", "Bob").Error
	})
	
	assert.NoError(t, err)
	
	// Verify data was committed
	var count int64
	db.Table("test_users").Count(&count)
	assert.Equal(t, int64(1), count)
	
	// Test that manager is accessible
	assert.NotNil(t, wrapper.Manager())
}