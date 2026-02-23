package logger

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey string

const (
	requestIdKey contextKey = "request_id"
)

type LoggerConfig struct {
	DebugLogLevel   bool
	RequestIDPrefix string
	FixedKeyValues  map[string]any
	ExtraFields     []string
}

type Logger struct {
	logger          *zap.SugaredLogger
	debugLogLevel   bool
	requestIDPrefix string
	fixedKeyValues  map[string]any
	extraFields     []string
}

func NewLogger(config LoggerConfig) *Logger {
	logger := &Logger{
		debugLogLevel:   config.DebugLogLevel,
		requestIDPrefix: config.RequestIDPrefix,
		fixedKeyValues:  config.FixedKeyValues,
		extraFields:     config.ExtraFields,
	}

	zapLoggerConfig := zap.NewProductionConfig()
	if logger.debugLogLevel {
		zapLoggerConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	zapLoggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	zapLoggerConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapLoggerConfig.EncoderConfig.TimeKey = "@timestamp"
	zapLoggerConfig.EncoderConfig.MessageKey = "message"

	zapLogger, err := zapLoggerConfig.Build(
		zap.AddCallerSkip(1),
	)

	if err != nil {
		panic(err)
	}

	logger.logger = zapLogger.Sugar()
	return logger
}

func (l *Logger) Debug(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Debugw(msg, combinedAttributes...)
}

func (l *Logger) Info(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Infow(msg, combinedAttributes...)
}

func (l *Logger) Warn(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Warnw(msg, combinedAttributes...)
}

func (l *Logger) Error(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Errorw(msg, combinedAttributes...)
}

func (l *Logger) Panic(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Panicw(msg, combinedAttributes...)
}

func (l *Logger) Fatal(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Fatalw(msg, combinedAttributes...)
}

func (l *Logger) Debugf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Debugw(msg, combinedAttributes...)
}

func (l *Logger) Infof(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Infow(msg, combinedAttributes...)
}

func (l *Logger) Warnf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Warnw(msg, combinedAttributes...)
}

func (l *Logger) Errorf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Errorw(msg, combinedAttributes...)
}

func (l *Logger) Panicf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Panicw(msg, combinedAttributes...)
}

func (l *Logger) Fatalf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := l.combineAttributes(ctx)
	l.logger.Fatalw(msg, combinedAttributes...)
}

func (l *Logger) Debugw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := l.combineAttributes(ctx, keysAndValues...)
	l.logger.Debugw(msg, combinedAttributes...)
}

func (l *Logger) Infow(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := l.combineAttributes(ctx, keysAndValues...)
	l.logger.Infow(msg, combinedAttributes...)
}

func (l *Logger) Warnw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := l.combineAttributes(ctx, keysAndValues...)
	l.logger.Warnw(msg, combinedAttributes...)
}

func (l *Logger) Errorw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := l.combineAttributes(ctx, keysAndValues...)
	l.logger.Errorw(msg, combinedAttributes...)
}

func (l *Logger) Panicw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := l.combineAttributes(ctx, keysAndValues...)
	l.logger.Panicw(msg, combinedAttributes...)
}

func (l *Logger) Fatalw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := l.combineAttributes(ctx, keysAndValues...)
	l.logger.Fatalw(msg, combinedAttributes...)
}

func (l *Logger) Flush() {
	l.logger.Sync()
}

func (l *Logger) IsDebugLogLevel() bool {
	return l.debugLogLevel
}

func (l *Logger) GetRequestIDPrefix() string {
	return l.requestIDPrefix
}

func (l *Logger) GetFixedKeyValues() map[string]any {
	return l.fixedKeyValues
}

func (l *Logger) GetExtraFieldsList() []string {
	return l.extraFields
}

func (l *Logger) GenerateRequestID() string {
	return l.requestIDPrefix + uuid.New().String()
}

func (l *Logger) SetRequestID(ctx context.Context, requestId string) context.Context {
	return context.WithValue(ctx, requestIdKey, requestId)
}

func (l *Logger) GetRequestID(ctx context.Context) (string, bool) {
	requestId, ok := ctx.Value(requestIdKey).(string)
	return requestId, ok
}

func (l *Logger) GetExtraFields(ctx context.Context) (map[string]any, bool) {
	if len(l.extraFields) == 0 {
		return nil, false
	}

	pairs := make(map[string]any)
	for _, field := range l.extraFields {
		if value := ctx.Value(field); value != nil {
			pairs[field] = value
		}
	}

	return pairs, true
}

func (l *Logger) combineAttributes(ctx context.Context, keysAndValues ...any) []any {
	var combined []any

	for k, v := range l.fixedKeyValues {
		combined = append(combined, k, v)
	}
	if requestId, ok := l.GetRequestID(ctx); ok {
		combined = append(combined, string(requestIdKey), requestId)
	}

	if extraFields, ok := l.GetExtraFields(ctx); ok {
		for k, v := range extraFields {
			combined = append(combined, k, v)
		}
	}

	combined = append(combined, keysAndValues...)
	return combined
}
