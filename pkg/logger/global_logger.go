package logger

import (
	"context"
	"fmt"
)

var loggerInstance *Logger

func init() {
	InitGlobalLogger(LoggerConfig{})
}

func InitGlobalLogger(config LoggerConfig) {
	loggerInstance = NewLogger(config)
}

func Debug(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Debugw(msg, combinedAttributes...)
}

func Info(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Infow(msg, combinedAttributes...)
}

func Warn(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Warnw(msg, combinedAttributes...)
}

func Error(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Errorw(msg, combinedAttributes...)
}

func Panic(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Panicw(msg, combinedAttributes...)
}

func Fatal(ctx context.Context, args ...any) {
	msg := fmt.Sprint(args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Fatalw(msg, combinedAttributes...)
}

func Debugf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Debugw(msg, combinedAttributes...)
}

func Infof(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Infow(msg, combinedAttributes...)
}

func Warnf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Warnw(msg, combinedAttributes...)
}

func Errorf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Errorw(msg, combinedAttributes...)
}

func Panicf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Panicw(msg, combinedAttributes...)
}

func Fatalf(ctx context.Context, template string, args ...any) {
	msg := fmt.Sprintf(template, args...)
	combinedAttributes := loggerInstance.combineAttributes(ctx)
	loggerInstance.logger.Fatalw(msg, combinedAttributes...)
}

func Debugw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := loggerInstance.combineAttributes(ctx, keysAndValues...)
	loggerInstance.logger.Debugw(msg, combinedAttributes...)
}

func Infow(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := loggerInstance.combineAttributes(ctx, keysAndValues...)
	loggerInstance.logger.Infow(msg, combinedAttributes...)
}

func Warnw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := loggerInstance.combineAttributes(ctx, keysAndValues...)
	loggerInstance.logger.Warnw(msg, combinedAttributes...)
}

func Errorw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := loggerInstance.combineAttributes(ctx, keysAndValues...)
	loggerInstance.logger.Errorw(msg, combinedAttributes...)
}

func Panicw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := loggerInstance.combineAttributes(ctx, keysAndValues...)
	loggerInstance.logger.Panicw(msg, combinedAttributes...)
}

func Fatalw(ctx context.Context, msg string, keysAndValues ...any) {
	combinedAttributes := loggerInstance.combineAttributes(ctx, keysAndValues...)
	loggerInstance.logger.Fatalw(msg, combinedAttributes...)
}

func Flush() {
	loggerInstance.Flush()
}

func IsDebugLogLevel() bool {
	return loggerInstance.IsDebugLogLevel()
}

func GetRequestIDPrefix() string {
	return loggerInstance.GetRequestIDPrefix()
}

func GetFixedKeyValues() map[string]any {
	return loggerInstance.GetFixedKeyValues()
}

func GetExtraFieldsList() []string {
	return loggerInstance.GetExtraFieldsList()
}

func GenerateRequestID() string {
	return loggerInstance.GenerateRequestID()
}

func SetRequestID(ctx context.Context, requestID string) context.Context {
	return loggerInstance.SetRequestID(ctx, requestID)
}

func GetRequestID(ctx context.Context) (string, bool) {
	return loggerInstance.GetRequestID(ctx)
}

func GetExtraFields(ctx context.Context) (map[string]any, bool) {
	return loggerInstance.GetExtraFields(ctx)
}
