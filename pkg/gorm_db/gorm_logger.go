package gormdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger implements gorm's logger.Interface and delegates all output
// to the project's structured JSON logger. It replaces the default GORM
// logger that writes coloured plain-text to stdout.
type GormLogger struct {
	// SlowThreshold defines the duration after which a SQL query is
	// considered "slow" and logged at WARN level. Zero disables slow-query
	// logging.
	SlowThreshold time.Duration

	// IgnoreRecordNotFoundError controls whether ErrRecordNotFound errors
	// are silenced in Trace output.
	IgnoreRecordNotFoundError bool

	// LogLevel mirrors gorm's LogLevel to control verbosity.
	LogLevel gormlogger.LogLevel
}

// NewGormLogger returns a GormLogger with sensible defaults:
//   - SlowThreshold: 200 ms
//   - LogLevel: Warn (logs errors and slow queries)
//   - IgnoreRecordNotFoundError: true
func NewGormLogger() *GormLogger {
	return &GormLogger{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
	}
}

// LogMode returns a new GormLogger with the requested verbosity.
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *l
	clone.LogLevel = level
	return &clone
}

// Info logs informational messages produced by GORM internals.
func (l *GormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.LogLevel >= gormlogger.Info {
		logger.Infow(ctx, fmt.Sprintf(msg, data...), "lib", "gorm")
	}
}

// Warn logs warning messages produced by GORM internals.
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.LogLevel >= gormlogger.Warn {
		logger.Warnw(ctx, fmt.Sprintf(msg, data...), "lib", "gorm")
	}
}

// Error logs error messages produced by GORM internals.
func (l *GormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.LogLevel >= gormlogger.Error {
		logger.Errorw(ctx, fmt.Sprintf(msg, data...), "lib", "gorm")
	}
}

// Trace is called by GORM after every SQL execution. It logs:
//   - errors at ERROR level (unless the error is ErrRecordNotFound and
//     IgnoreRecordNotFoundError is set)
//   - slow queries at WARN level
//   - all other queries at DEBUG level (only when LogLevel == Info)
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	elapsedMs := float64(elapsed.Nanoseconds()) / 1e6

	switch {
	// ── Query returned an error ──
	case err != nil && l.LogLevel >= gormlogger.Error &&
		(!errors.Is(err, gormlogger.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):

		sql, rows := fc()
		logger.Errorw(ctx, err.Error(),
			"lib", "gorm",
			"error", err.Error(),
			"elapsedMs", elapsedMs,
			"rows", rows,
			"sql", sql,
		)

	// ── Slow query ──
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= gormlogger.Warn:
		sql, rows := fc()
		logger.Warnw(ctx, fmt.Sprintf("gorm slow query >= %v", l.SlowThreshold),
			"lib", "gorm",
			"elapsedMs", elapsedMs,
			"rows", rows,
			"slowThreshold", l.SlowThreshold.String(),
			"sql", sql,
		)

	// ── Normal query (only at Info / debug verbosity) ──
	case l.LogLevel == gormlogger.Info:
		sql, rows := fc()
		logger.Debugw(ctx, "gorm query",
			"lib", "gorm",
			"elapsedMs", elapsedMs,
			"rows", rows,
			"sql", sql,
		)
	}
}
