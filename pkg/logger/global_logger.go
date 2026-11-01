package logger

import (
	"context"
	"sync/atomic"

	"go.uber.org/zap/zapcore"
)

// global holds the process-wide *Logger. It is stored in an atomic.Pointer so
// that InitGlobalLogger can replace it concurrently with in-flight log calls
// without data races.
var global atomic.Pointer[Logger]

func init() {
	InitGlobalLogger(Config{})
}

// InitGlobalLogger builds a new global logger from config and installs it
// atomically. Call it once at program start-up, before spawning goroutines
// that log — although replacing it later is race-free, doing so mid-flight can
// interleave configuration.
func InitGlobalLogger(config Config) {
	// The global wrapper functions add one frame between the user and the
	// instance method, so compensate for accurate caller info.
	config.CallerSkip++
	global.Store(NewLogger(config))
}

// SetGlobalLogger installs an already-constructed logger as the global one.
// Note: caller-skip is not adjusted, so the caller annotation may point one
// frame off when used through the package-level functions. Prefer
// InitGlobalLogger unless you specifically need to share an instance.
func SetGlobalLogger(l *Logger) {
	global.Store(l)
}

// L returns the current global *Logger. It never returns nil.
func L() *Logger {
	return global.Load()
}

func Debug(ctx context.Context, args ...any) { global.Load().Debug(ctx, args...) }
func Info(ctx context.Context, args ...any)  { global.Load().Info(ctx, args...) }
func Warn(ctx context.Context, args ...any)  { global.Load().Warn(ctx, args...) }
func Error(ctx context.Context, args ...any) { global.Load().Error(ctx, args...) }
func Panic(ctx context.Context, args ...any) { global.Load().Panic(ctx, args...) }
func Fatal(ctx context.Context, args ...any) { global.Load().Fatal(ctx, args...) }

func Debugf(ctx context.Context, template string, args ...any) {
	global.Load().Debugf(ctx, template, args...)
}

func Infof(ctx context.Context, template string, args ...any) {
	global.Load().Infof(ctx, template, args...)
}

func Warnf(ctx context.Context, template string, args ...any) {
	global.Load().Warnf(ctx, template, args...)
}

func Errorf(ctx context.Context, template string, args ...any) {
	global.Load().Errorf(ctx, template, args...)
}

func Panicf(ctx context.Context, template string, args ...any) {
	global.Load().Panicf(ctx, template, args...)
}

func Fatalf(ctx context.Context, template string, args ...any) {
	global.Load().Fatalf(ctx, template, args...)
}

func Debugw(ctx context.Context, msg string, keysAndValues ...any) {
	global.Load().Debugw(ctx, msg, keysAndValues...)
}

func Infow(ctx context.Context, msg string, keysAndValues ...any) {
	global.Load().Infow(ctx, msg, keysAndValues...)
}

func Warnw(ctx context.Context, msg string, keysAndValues ...any) {
	global.Load().Warnw(ctx, msg, keysAndValues...)
}

func Errorw(ctx context.Context, msg string, keysAndValues ...any) {
	global.Load().Errorw(ctx, msg, keysAndValues...)
}

func Panicw(ctx context.Context, msg string, keysAndValues ...any) {
	global.Load().Panicw(ctx, msg, keysAndValues...)
}

func Fatalw(ctx context.Context, msg string, keysAndValues ...any) {
	global.Load().Fatalw(ctx, msg, keysAndValues...)
}

// Flush flushes the global logger's buffered entries.
func Flush() error {
	return global.Load().Flush()
}

// SetLevel changes the global logger's minimum level at runtime.
func SetLevel(level zapcore.Level) {
	global.Load().SetLevel(level)
}

// GetLevel returns the global logger's current minimum level.
func GetLevel() zapcore.Level {
	return global.Load().GetLevel()
}

// Enabled reports whether level would be logged by the global logger.
func Enabled(level zapcore.Level) bool {
	return global.Load().Enabled(level)
}

func IsDebugLogLevel() bool {
	return global.Load().IsDebugLogLevel()
}

func GetRequestIDPrefix() string {
	return global.Load().GetRequestIDPrefix()
}

func GetFixedKeyValues() map[string]any {
	return global.Load().GetFixedKeyValues()
}

func GetExtraFieldsList() []string {
	return global.Load().GetExtraFieldsList()
}

func GenerateRequestID() string {
	return global.Load().GenerateRequestID()
}

func SetRequestID(ctx context.Context, requestID string) context.Context {
	return global.Load().SetRequestID(ctx, requestID)
}

func GetRequestID(ctx context.Context) (string, bool) {
	return global.Load().GetRequestID(ctx)
}

func GetExtraFields(ctx context.Context) (map[string]any, bool) {
	return global.Load().GetExtraFields(ctx)
}
