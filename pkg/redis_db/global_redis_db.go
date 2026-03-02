package redisdb

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// defaultRedisClient is the package-level RedisClient used by all
// top-level convenience functions (GetClient, GlobalConnect, etc.).
var defaultRedisClient *RedisClient

func init() {
	defaultRedisClient = &RedisClient{
		clientName: "globalRedisClient",
	}
}

// GetClient returns the underlying client of the global RedisClient.
func GetClient() redis.UniversalClient {
	return defaultRedisClient.GetClient()
}

// SetClient replaces the underlying client of the global RedisClient.
func SetClient(client redis.UniversalClient) {
	defaultRedisClient.SetClient(client)
}

// GetConfig returns a copy of the global RedisClient's config.
func GetConfig() Config {
	return defaultRedisClient.GetConfig()
}

// SetConfig replaces the global RedisClient's config.
// Call GlobalConnect afterwards to apply it.
func SetConfig(config Config) {
	defaultRedisClient.SetConfig(config)
}

// GetClientName returns the logical name of the global RedisClient.
func GetClientName() string {
	return defaultRedisClient.GetClientName()
}

// SetClientName changes the logical name of the global RedisClient.
func SetClientName(name string) {
	defaultRedisClient.SetClientName(name)
}

// GlobalConnect establishes the global Redis connection using the stored
// config. Call SetConfig first to configure connection parameters.
func GlobalConnect(ctx context.Context) error {
	return defaultRedisClient.Connect(ctx)
}

// GlobalReconnect closes and re-establishes the global Redis connection.
func GlobalReconnect(ctx context.Context) error {
	return defaultRedisClient.Reconnect(ctx)
}

// GlobalClose gracefully shuts down the global Redis client.
func GlobalClose() error {
	return defaultRedisClient.Close()
}
