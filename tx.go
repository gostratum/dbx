package dbx

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// WithTx executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func WithTx(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.Transaction(fn)
}

// WithTxContext executes a function within a database transaction with context
func WithTxContext(ctx context.Context, db *gorm.DB, fn func(ctx context.Context, tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, tx)
	})
}

// TxManager provides transaction management utilities
type TxManager struct {
	db *gorm.DB
}

// NewTxManager creates a new transaction manager
func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

// Begin starts a new transaction
func (tm *TxManager) Begin() *gorm.DB {
	return tm.db.Begin()
}

// BeginContext starts a new transaction with context
func (tm *TxManager) BeginContext(ctx context.Context) *gorm.DB {
	return tm.db.WithContext(ctx).Begin()
}

// Commit commits the transaction
func (tm *TxManager) Commit(tx *gorm.DB) error {
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction
func (tm *TxManager) Rollback(tx *gorm.DB) error {
	if err := tx.Rollback().Error; err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}

// WithTx executes a function within a transaction managed by TxManager
func (tm *TxManager) WithTx(fn func(tx *gorm.DB) error) error {
	return WithTx(tm.db, fn)
}

// WithTxContext executes a function within a transaction with context
func (tm *TxManager) WithTxContext(ctx context.Context, fn func(ctx context.Context, tx *gorm.DB) error) error {
	return WithTxContext(ctx, tm.db, fn)
}

// SavePoint creates a savepoint within the current transaction
func (tm *TxManager) SavePoint(tx *gorm.DB, name string) error {
	if err := tx.SavePoint(name).Error; err != nil {
		return fmt.Errorf("failed to create savepoint %s: %w", name, err)
	}
	return nil
}

// RollbackTo rolls back to a savepoint
func (tm *TxManager) RollbackTo(tx *gorm.DB, name string) error {
	if err := tx.RollbackTo(name).Error; err != nil {
		return fmt.Errorf("failed to rollback to savepoint %s: %w", name, err)
	}
	return nil
}

// TxWrapper wraps a database connection with transaction utilities
type TxWrapper struct {
	*gorm.DB
	manager *TxManager
}

// NewTxWrapper creates a new transaction wrapper
func NewTxWrapper(db *gorm.DB) *TxWrapper {
	return &TxWrapper{
		DB:      db,
		manager: NewTxManager(db),
	}
}

// WithTx executes a function within a transaction
func (tw *TxWrapper) WithTx(fn func(tx *gorm.DB) error) error {
	return tw.manager.WithTx(fn)
}

// WithTxContext executes a function within a transaction with context
func (tw *TxWrapper) WithTxContext(ctx context.Context, fn func(ctx context.Context, tx *gorm.DB) error) error {
	return tw.manager.WithTxContext(ctx, fn)
}

// Manager returns the underlying transaction manager
func (tw *TxWrapper) Manager() *TxManager {
	return tw.manager
}
