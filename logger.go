package dbx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// zapGormLogger is a zap-based logger for GORM
type zapGormLogger struct {
	zap           *zap.Logger
	logLevel      gormlogger.LogLevel
	slowThreshold time.Duration
}

// NewGormLogger creates a new GORM logger using zap
func NewGormLogger(logger *zap.Logger, logLevel string, slowThreshold time.Duration) gormlogger.Interface {
	var level gormlogger.LogLevel
	
	switch logLevel {
	case "silent":
		level = gormlogger.Silent
	case "error":
		level = gormlogger.Error
	case "warn":
		level = gormlogger.Warn
	case "info":
		level = gormlogger.Info
	default:
		level = gormlogger.Warn
	}

	return &zapGormLogger{
		zap:           logger.Named("gorm"),
		logLevel:      level,
		slowThreshold: slowThreshold,
	}
}

// LogMode sets the log level
func (l *zapGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

// Info logs info level messages
func (l *zapGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Info {
		l.zap.Info(fmt.Sprintf(msg, data...), l.contextFields(ctx)...)
	}
}

// Warn logs warn level messages
func (l *zapGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Warn {
		l.zap.Warn(fmt.Sprintf(msg, data...), l.contextFields(ctx)...)
	}
}

// Error logs error level messages
func (l *zapGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Error {
		l.zap.Error(fmt.Sprintf(msg, data...), l.contextFields(ctx)...)
	}
}

// Trace logs SQL traces
func (l *zapGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.logLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()
	
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.String("sql", sql),
		zap.Int64("rows", rows),
	}
	fields = append(fields, l.contextFields(ctx)...)

	switch {
	case err != nil && l.logLevel >= gormlogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound)):
		fields = append(fields, zap.Error(err))
		l.zap.Error("SQL execution failed", fields...)
	case elapsed > l.slowThreshold && l.slowThreshold != 0 && l.logLevel >= gormlogger.Warn:
		fields = append(fields, zap.Duration("slow_threshold", l.slowThreshold))
		l.zap.Warn("Slow SQL query detected", fields...)
	case l.logLevel == gormlogger.Info:
		l.zap.Info("SQL query executed", fields...)
	}
}

// contextFields extracts logging fields from context
func (l *zapGormLogger) contextFields(ctx context.Context) []zap.Field {
	var fields []zap.Field
	
	// Extract trace ID if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			fields = append(fields, zap.String("trace_id", id))
		}
	}
	
	// Extract request ID if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			fields = append(fields, zap.String("request_id", id))
		}
	}
	
	// Extract user ID if available
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			fields = append(fields, zap.String("user_id", id))
		}
	}
	
	return fields
}