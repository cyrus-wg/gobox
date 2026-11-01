// Package logger provides a thin, opinionated wrapper around Uber's zap
// library that emits structured JSON logs suitable for production services.
//
// It offers two usage styles that share the same implementation:
//
//   - An instance logger (NewLogger) for dependency injection and per-component
//     configuration.
//   - A process-wide global logger (InitGlobalLogger + package-level functions)
//     for convenience.
//
// Every log call takes a context.Context as its first argument. Values such as
// the request ID (SetRequestID) and any configured ExtraFields are pulled from
// the context automatically and attached to each entry.
package logger

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// contextKey is an unexported type used for context keys defined in this
// package. Using an unexported type prevents collisions with keys defined in
// other packages.
type contextKey string

// requestIDKey is the context key under which the request ID is stored.
const requestIDKey contextKey = "request_id"

// Encoding selects the on-the-wire log format.
type Encoding string

const (
	// EncodingJSON emits one JSON object per log line (default, production).
	EncodingJSON Encoding = "json"
	// EncodingConsole emits human-readable, coloured console output (dev).
	EncodingConsole Encoding = "console"
)

// Config controls how a Logger is constructed. The zero value is valid and
// yields a production JSON logger at INFO level writing to stderr.
type Config struct {
	// Level is the minimum enabled level. Defaults to InfoLevel.
	// When DebugLogLevel is true it is forced to DebugLevel (kept for
	// backwards compatibility).
	Level zapcore.Level

	// DebugLogLevel, when true, sets the level to Debug. Deprecated: prefer
	// setting Level directly; retained for compatibility.
	DebugLogLevel bool

	// RequestIDPrefix is prepended to IDs produced by GenerateRequestID.
	RequestIDPrefix string

	// FixedKeyValues are attached to every log entry emitted by this logger.
	FixedKeyValues map[string]any

	// ExtraFields lists context keys whose values are pulled from the
	// context and attached to each entry when present.
	ExtraFields []string

	// Encoding selects json (default) or console output.
	Encoding Encoding

	// OutputPaths are zap output paths (e.g. "stderr", "stdout", or a file
	// path). Defaults to ["stderr"].
	OutputPaths []string

	// DisableCaller omits the caller (file:line) annotation when true.
	DisableCaller bool

	// DisableStacktrace omits automatic stack traces on Error+ when true.
	DisableStacktrace bool

	// DisableSampling turns off zap's production sampler. Sampling reduces
	// log volume under bursts; disable it when you must not drop entries.
	DisableSampling bool

	// CallerSkip adds extra caller frames to skip. Most callers should leave
	// this at zero.
	CallerSkip int
}

// LoggerConfig is a backwards-compatible alias for Config.
//
// Deprecated: use Config.
type LoggerConfig = Config

// Logger is a structured logger. It is safe for concurrent use.
type Logger struct {
	sugar           *zap.SugaredLogger
	level           zap.AtomicLevel
	requestIDPrefix string
	fixedKeyValues  map[string]any
	extraFields     []string
}

// NewLogger builds a Logger from config. It panics only if zap itself fails to
// build, which indicates a programming error (e.g. an invalid output path).
func NewLogger(config Config) *Logger {
	level := config.Level
	if config.DebugLogLevel {
		level = zapcore.DebugLevel
	}
	atomicLevel := zap.NewAtomicLevelAt(level)

	encoding := config.Encoding
	if encoding == "" {
		encoding = EncodingJSON
	}

	outputPaths := config.OutputPaths
	if len(outputPaths) == 0 {
		outputPaths = []string{"stderr"}
	}

	zapConfig := zap.NewProductionConfig()
	zapConfig.Level = atomicLevel
	zapConfig.Encoding = string(encoding)
	zapConfig.OutputPaths = outputPaths
	zapConfig.DisableCaller = config.DisableCaller
	zapConfig.DisableStacktrace = config.DisableStacktrace
	if config.DisableSampling {
		zapConfig.Sampling = nil
	}

	zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapConfig.EncoderConfig.TimeKey = "@timestamp"
	zapConfig.EncoderConfig.MessageKey = "message"

	// One wrapper frame (Logger.X or the package-level X) sits between the
	// user's call site and zap, so skip it plus any caller-requested frames.
	built, err := zapConfig.Build(zap.AddCallerSkip(1 + config.CallerSkip))
	if err != nil {
		panic(fmt.Errorf("logger: failed to build zap logger: %w", err))
	}

	return &Logger{
		sugar:           built.Sugar(),
		level:           atomicLevel,
		requestIDPrefix: config.RequestIDPrefix,
		fixedKeyValues:  config.FixedKeyValues,
		extraFields:     config.ExtraFields,
	}
}

// With returns a child logger that includes the supplied key-value pairs on
// every entry. The parent is unaffected. keysAndValues follows the same
// alternating key/value convention as the *w methods.
func (l *Logger) With(keysAndValues ...any) *Logger {
	if len(keysAndValues) == 0 {
		return l
	}
	clone := *l
	clone.sugar = l.sugar.With(keysAndValues...)
	return &clone
}

// Named adds a sub-scope name to the logger, appended with a dot separator.
func (l *Logger) Named(name string) *Logger {
	clone := *l
	clone.sugar = l.sugar.Named(name)
	return &clone
}

func (l *Logger) Debug(ctx context.Context, args ...any) {
	l.sugar.Debugw(fmt.Sprint(args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Info(ctx context.Context, args ...any) {
	l.sugar.Infow(fmt.Sprint(args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Warn(ctx context.Context, args ...any) {
	l.sugar.Warnw(fmt.Sprint(args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Error(ctx context.Context, args ...any) {
	l.sugar.Errorw(fmt.Sprint(args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Panic(ctx context.Context, args ...any) {
	l.sugar.Panicw(fmt.Sprint(args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Fatal(ctx context.Context, args ...any) {
	l.sugar.Fatalw(fmt.Sprint(args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Debugf(ctx context.Context, template string, args ...any) {
	l.sugar.Debugw(fmt.Sprintf(template, args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Infof(ctx context.Context, template string, args ...any) {
	l.sugar.Infow(fmt.Sprintf(template, args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Warnf(ctx context.Context, template string, args ...any) {
	l.sugar.Warnw(fmt.Sprintf(template, args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Errorf(ctx context.Context, template string, args ...any) {
	l.sugar.Errorw(fmt.Sprintf(template, args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Panicf(ctx context.Context, template string, args ...any) {
	l.sugar.Panicw(fmt.Sprintf(template, args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Fatalf(ctx context.Context, template string, args ...any) {
	l.sugar.Fatalw(fmt.Sprintf(template, args...), l.combineAttributes(ctx)...)
}

func (l *Logger) Debugw(ctx context.Context, msg string, keysAndValues ...any) {
	l.sugar.Debugw(msg, l.combineAttributes(ctx, keysAndValues...)...)
}

func (l *Logger) Infow(ctx context.Context, msg string, keysAndValues ...any) {
	l.sugar.Infow(msg, l.combineAttributes(ctx, keysAndValues...)...)
}

func (l *Logger) Warnw(ctx context.Context, msg string, keysAndValues ...any) {
	l.sugar.Warnw(msg, l.combineAttributes(ctx, keysAndValues...)...)
}

func (l *Logger) Errorw(ctx context.Context, msg string, keysAndValues ...any) {
	l.sugar.Errorw(msg, l.combineAttributes(ctx, keysAndValues...)...)
}

func (l *Logger) Panicw(ctx context.Context, msg string, keysAndValues ...any) {
	l.sugar.Panicw(msg, l.combineAttributes(ctx, keysAndValues...)...)
}

func (l *Logger) Fatalw(ctx context.Context, msg string, keysAndValues ...any) {
	l.sugar.Fatalw(msg, l.combineAttributes(ctx, keysAndValues...)...)
}

// Flush writes any buffered log entries. Call it before the process exits.
// The underlying sync error is returned but is commonly ignorable for
// stderr/stdout on some platforms.
func (l *Logger) Flush() error {
	return l.sugar.Sync()
}

// SetLevel changes the minimum enabled level at runtime. It affects this
// logger and every child created from it via With/Named, because they share
// the same AtomicLevel.
func (l *Logger) SetLevel(level zapcore.Level) {
	l.level.SetLevel(level)
}

// GetLevel returns the current minimum enabled level.
func (l *Logger) GetLevel() zapcore.Level {
	return l.level.Level()
}

// Enabled reports whether the given level would currently be logged.
func (l *Logger) Enabled(level zapcore.Level) bool {
	return l.level.Enabled(level)
}

// IsDebugLogLevel reports whether the debug level is currently enabled.
func (l *Logger) IsDebugLogLevel() bool {
	return l.level.Enabled(zapcore.DebugLevel)
}

// GetRequestIDPrefix returns the configured request-ID prefix.
func (l *Logger) GetRequestIDPrefix() string {
	return l.requestIDPrefix
}

// GetFixedKeyValues returns the configured fixed key-value pairs.
func (l *Logger) GetFixedKeyValues() map[string]any {
	return l.fixedKeyValues
}

// GetExtraFieldsList returns the configured extra field context keys.
func (l *Logger) GetExtraFieldsList() []string {
	return l.extraFields
}

// GenerateRequestID returns a new request ID: the configured prefix followed
// by a random UUIDv4.
func (l *Logger) GenerateRequestID() string {
	return l.requestIDPrefix + uuid.NewString()
}

// SetRequestID returns a copy of ctx carrying the given request ID.
func (l *Logger) SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID returns the request ID stored in ctx, if any.
func (l *Logger) GetRequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey).(string)
	return requestID, ok
}

// GetExtraFields returns the configured ExtraFields that are present on ctx.
// The bool is false only when no ExtraFields are configured at all.
func (l *Logger) GetExtraFields(ctx context.Context) (map[string]any, bool) {
	if len(l.extraFields) == 0 {
		return nil, false
	}

	pairs := make(map[string]any, len(l.extraFields))
	for _, field := range l.extraFields {
		//nolint:staticcheck // ExtraFields intentionally uses caller-supplied
		// string keys so values set with context.WithValue(ctx, "k", v) resolve.
		if value := ctx.Value(field); value != nil {
			pairs[field] = value
		}
	}

	return pairs, true
}

// combineAttributes merges fixed key-values, the request ID, configured extra
// fields, and per-call keysAndValues into a single slice for zap.
func (l *Logger) combineAttributes(ctx context.Context, keysAndValues ...any) []any {
	// Pre-size to reduce reallocations.
	combined := make([]any, 0, len(l.fixedKeyValues)*2+len(l.extraFields)*2+2+len(keysAndValues))

	for k, v := range l.fixedKeyValues {
		combined = append(combined, k, v)
	}
	if requestID, ok := l.GetRequestID(ctx); ok {
		combined = append(combined, string(requestIDKey), requestID)
	}
	if extraFields, ok := l.GetExtraFields(ctx); ok {
		for k, v := range extraFields {
			combined = append(combined, k, v)
		}
	}

	combined = append(combined, keysAndValues...)
	return combined
}
