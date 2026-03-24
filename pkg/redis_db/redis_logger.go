package redisdb

import (
	"context"
	"fmt"

	"github.com/cyrus-wg/gobox/pkg/logger"
)

// RedisLogger implements the go-redis internal.Logging interface so that all
// log output produced by the redis client library (e.g. Sentinel discovery,
// failover events) is routed through the project's structured JSON logger
// instead of Go's default log package which writes plain-text to stderr.
//
// Usage: pass an instance to redis.SetLogger before creating clients.
//
//	redis.SetLogger(redisdb.NewRedisLogger())
type RedisLogger struct{}

// NewRedisLogger returns a RedisLogger ready for use with redis.SetLogger.
func NewRedisLogger() *RedisLogger {
	return &RedisLogger{}
}

// Printf satisfies the go-redis internal.Logging interface.
// All messages are logged at INFO level with a "lib":"go-redis" tag so they
// can be easily filtered.
func (l *RedisLogger) Printf(ctx context.Context, format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	logger.Infow(ctx, msg, "lib", "go-redis")
}
