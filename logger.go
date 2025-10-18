package dbx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gostratum/core/logx"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type gormLoggerAdapter struct {
	logger        logx.Logger
	logLevel      gormlogger.LogLevel
	slowThreshold time.Duration
}

// NewGormLogger creates a new GORM logger using zap
func NewGormLogger(logger logx.Logger, logLevel string, slowThreshold time.Duration) gormlogger.Interface {
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

	// approximate zap.Named by adding a component field via With
	return &gormLoggerAdapter{
		logger:        logger.With(logx.String("component", "gorm")),
		logLevel:      level,
		slowThreshold: slowThreshold,
	}
}

// LogMode sets the log level
func (l *gormLoggerAdapter) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

// Info logs info level messages
func (l *gormLoggerAdapter) Info(ctx context.Context, msg string, data ...any) {
	if l.logLevel >= gormlogger.Info {
		l.logger.Info(fmt.Sprintf(msg, data...), l.contextFields(ctx)...)
	}
}

// Warn logs warn level messages
func (l *gormLoggerAdapter) Warn(ctx context.Context, msg string, data ...any) {
	if l.logLevel >= gormlogger.Warn {
		l.logger.Warn(fmt.Sprintf(msg, data...), l.contextFields(ctx)...)
	}
}

// Error logs error level messages
func (l *gormLoggerAdapter) Error(ctx context.Context, msg string, data ...any) {
	if l.logLevel >= gormlogger.Error {
		l.logger.Error(fmt.Sprintf(msg, data...), l.contextFields(ctx)...)
	}
}

// Trace logs SQL traces
func (l *gormLoggerAdapter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.logLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []logx.Field{
		logx.Duration("elapsed", elapsed),
		logx.String("sql", sql),
		logx.Int64("rows", rows),
	}
	fields = append(fields, l.contextFields(ctx)...)

	switch {
	case err != nil && l.logLevel >= gormlogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound)):
		fields = append(fields, logx.Err(err))
		l.logger.Error("SQL execution failed", fields...)
	case elapsed > l.slowThreshold && l.slowThreshold != 0 && l.logLevel >= gormlogger.Warn:
		fields = append(fields, logx.Duration("slow_threshold", l.slowThreshold))
		l.logger.Warn("Slow SQL query detected", fields...)
	case l.logLevel == gormlogger.Info:
		l.logger.Info("SQL query executed", fields...)
	}
}

// contextFields extracts logging fields from context
func (l *gormLoggerAdapter) contextFields(ctx context.Context) []logx.Field {
	var fields []logx.Field

	// Extract trace ID if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			fields = append(fields, logx.String("trace_id", id))
		}
	}

	// Extract request ID if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			fields = append(fields, logx.String("request_id", id))
		}
	}

	// Extract user ID if available
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			fields = append(fields, logx.String("user_id", id))
		}
	}

	return fields
}
