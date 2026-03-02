package redisdb

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// GetClient returns the underlying redis.UniversalClient.
// Returns nil if Connect has not been called or Close has been called.
func (rc *RedisClient) GetClient() redis.UniversalClient {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.client
}

// SetClient replaces the underlying redis.UniversalClient.
// The caller is responsible for closing the previously set client if needed.
func (rc *RedisClient) SetClient(client redis.UniversalClient) {
	rc.mu.Lock()
	rc.client = client
	rc.mu.Unlock()
}

// GetConfig returns a copy of the current connection config.
func (rc *RedisClient) GetConfig() Config {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.redisConfig
}

// SetConfig replaces the stored config. This does not reconnect —
// call Connect or Reconnect afterwards to apply the new config.
func (rc *RedisClient) SetConfig(config Config) {
	rc.mu.Lock()
	rc.redisConfig = config
	rc.mu.Unlock()
}

// GetClientName returns the logical name of this client (used in log messages).
func (rc *RedisClient) GetClientName() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.clientName
}

// SetClientName changes the logical name of this client.
func (rc *RedisClient) SetClientName(name string) {
	rc.mu.Lock()
	rc.clientName = name
	rc.mu.Unlock()
}

// Connect establishes a connection to Redis using the stored config.
// It applies defaults to zero-value config fields, validates pool settings,
// creates the client, and verifies connectivity with PING.
func (rc *RedisClient) Connect(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
		logger.Warnw(ctx, "Context is nil, using context.Background() in Redis Connect", "clientName", rc.GetClientName())
	}

	config := applyDefaults(rc.GetConfig())
	rc.SetConfig(config) // persist applied defaults for future reference

	if len(config.Addrs) == 0 {
		return errors.New("redisdb: at least one address is required in Config.Addrs")
	}

	// Validate pool hierarchy. go-redis silently clamps invalid values which
	// masks misconfiguration — fail explicitly instead.
	if config.MinIdleConns > config.MaxIdleConns {
		return errors.New("redisdb: MinIdleConns must be ≤ MaxIdleConns")
	}
	if config.MaxIdleConns > config.PoolSize {
		return errors.New("redisdb: MaxIdleConns must be ≤ PoolSize")
	}

	logger.Infow(ctx, "Connecting to Redis", "clientName", rc.GetClientName())

	options := &redis.UniversalOptions{
		Addrs:    config.Addrs,
		Password: config.Password,

		// Timeouts — -1 passes through to go-redis as "no deadline"
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,

		// Connection pool
		PoolSize:        config.PoolSize,
		MinIdleConns:    config.MinIdleConns,
		MaxIdleConns:    config.MaxIdleConns,
		ConnMaxIdleTime: config.ConnMaxIdleTime,
		ConnMaxLifetime: config.ConnMaxLifetime,

		// Retry — -1 disables retries entirely in go-redis v9
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: config.MinRetryBackoff,
		MaxRetryBackoff: config.MaxRetryBackoff,
	}

	// Sentinel mode: requires MasterName
	if config.MasterName != "" {
		options.MasterName = config.MasterName
		logger.Infow(ctx, "Using Redis Sentinel", "masterName", config.MasterName, "clientName", rc.GetClientName())
	}

	// DB selection is only supported for standalone and Sentinel modes.
	// Cluster mode is auto-detected: multiple addresses with no MasterName.
	isClusterMode := len(config.Addrs) > 1 && config.MasterName == ""
	if !isClusterMode {
		options.DB = config.DB
		logger.Infow(ctx, "Using Redis DB", "db", config.DB, "clientName", rc.GetClientName())
	} else {
		logger.Infow(ctx, "Using Redis Cluster mode (DB selection not supported, using DB 0)", "clientName", rc.GetClientName())
	}

	if config.TLSEnabled {
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		logger.Infow(ctx, "TLS is enabled for Redis connection", "clientName", rc.GetClientName())
	}

	rc.SetClient(redis.NewUniversalClient(options))

	// Verify the connection is alive before returning.
	// Use GetClient() to go through the mutex, not rc.client directly.
	if err := rc.GetClient().Ping(ctx).Err(); err != nil {
		rc.Close() // close client if ping fails
		logger.Errorw(ctx, "Failed to connect to Redis", "error", err, "clientName", rc.GetClientName())
		return err
	}

	logger.Infow(ctx, "Redis connection established successfully", "clientName", rc.GetClientName())
	return nil
}

// Reconnect closes the existing connection and re-establishes it using the
// stored config. Use this after a topology change or config update.
func (rc *RedisClient) Reconnect(ctx context.Context) error {
	logger.Infow(ctx, "Reconnecting to Redis", "clientName", rc.GetClientName())
	if err := rc.Close(); err != nil {
		logger.Errorw(ctx, "Error closing existing Redis client during reconnect", "error", err, "clientName", rc.GetClientName())
		// Continue with reconnect attempt even if close fails, as the old client may be in a bad state.
	}

	return rc.Connect(ctx)
}

// Close gracefully shuts down the Redis client and releases all pool
// resources. The client is set to nil after closing. Safe to call
// concurrently. Returns nil if no client is set.
func (rc *RedisClient) Close() error {
	rc.mu.Lock()
	client := rc.client
	rc.client = nil
	rc.mu.Unlock()

	if client != nil {
		return client.Close()
	}
	return nil
}

// applyDefaults returns a copy of config with sensible defaults filled in for
// any zero-value fields. A value of -1 means "explicitly disabled" and is
// preserved. This is called at the start of Connect.
func applyDefaults(config Config) Config {
	// Timeouts — only apply default when field is 0 (unset).
	// -1 means "caller explicitly disabled this timeout" and must be preserved.
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 3 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 3 * time.Second
	}

	// Pool — MinIdleConns ≤ MaxIdleConns ≤ PoolSize
	if config.PoolSize == 0 {
		config.PoolSize = 10
	}
	if config.MinIdleConns == 0 {
		config.MinIdleConns = 2
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 5 * time.Minute
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 30 * time.Minute
	}

	// Retry — -1 means "caller disabled retries" and must be preserved.
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.MinRetryBackoff == 0 {
		config.MinRetryBackoff = 8 * time.Millisecond
	}
	if config.MaxRetryBackoff == 0 {
		config.MaxRetryBackoff = 512 * time.Millisecond
	}

	return config
}
