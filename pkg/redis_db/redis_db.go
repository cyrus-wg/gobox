package redisdb

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/redis/go-redis/v9"
)

var (
	defaultClient redis.UniversalClient
	defaultMu     sync.RWMutex // protects defaultClient for concurrent access
)

// GetClient returns the package-level Redis client.
// Returns nil if GlobalConnect has not been called yet.
func GetClient() redis.UniversalClient {
	defaultMu.RLock()
	defer defaultMu.RUnlock()

	return defaultClient
}

// SetClient replaces the package-level Redis client. It is safe to call
// concurrently. The caller is responsible for closing any previously set
// client if it is no longer needed.
//
// Prefer creating separate clients via Connect when different parts of
// your application need different Redis configurations.
func SetClient(client redis.UniversalClient) {
	defaultMu.Lock()
	defaultClient = client
	defaultMu.Unlock()
}

// Config holds all settings for opening a Redis connection.
// Zero values are replaced with production-ready defaults via applyDefaults.
//
// Timeout and backoff fields use time.Duration so callers can express values
// naturally (e.g. 5*time.Second) and the special value -1 disables the
// corresponding timeout/retry.
type Config struct {
	// Addrs is the list of Redis server addresses (host:port).
	//   - Single address                  → standalone mode
	//   - Multiple addresses + MasterName → Sentinel mode
	//   - Multiple addresses (no master)  → Cluster mode
	Addrs      []string
	Password   string
	DB         int    // ignored in Cluster mode
	MasterName string // non-empty enables Sentinel mode
	TLSEnabled bool

	// DialTimeout is the max time to establish a TCP connection (including TLS).
	// Set to -1 to disable. Default: 5s.
	DialTimeout time.Duration

	// ReadTimeout is the per-command deadline for reading a response from server.
	// Set to -1 to disable — required for blocking commands (BLPOP, SUBSCRIBE,
	// XREAD BLOCK). When disabled, always bound calls with a context deadline.
	// Default: 3s.
	ReadTimeout time.Duration

	// WriteTimeout is the per-command deadline for sending command bytes.
	// Set to -1 to disable. Default: 3s.
	WriteTimeout time.Duration

	// PoolSize is the total pool capacity: the maximum number of socket
	// connections (idle + in-use combined) maintained per node.
	// Default: 10.
	PoolSize int

	// MinIdleConns is the number of idle connections pre-warmed at startup
	// and maintained even during quiet periods.
	// Must satisfy: MinIdleConns ≤ MaxIdleConns ≤ PoolSize.
	// Default: 2.
	MinIdleConns int

	// MaxIdleConns is the maximum number of idle connections retained in the
	// pool after a load spike. Excess idle connections are closed.
	// Must satisfy: MinIdleConns ≤ MaxIdleConns ≤ PoolSize.
	// Default: 5.
	MaxIdleConns int

	// ConnMaxIdleTime is how long a connection may sit idle before being closed.
	// Shorter values free server-side resources faster during quiet periods.
	// Default: 5 minutes.
	ConnMaxIdleTime time.Duration

	// ConnMaxLifetime is the maximum age of any connection regardless of activity.
	// Forces periodic recycling to recover from stale/broken connections.
	// 0 means no max lifetime. Default: 30 minutes.
	ConnMaxLifetime time.Duration

	// MaxRetries is the number of retries on transient failures.
	// Set to -1 to disable all retries.
	// Default: 3.
	MaxRetries int

	// MinRetryBackoff is the minimum wait between retries (jittered).
	// Set to -1 to disable backoff (retry immediately).
	// Default: 8ms.
	MinRetryBackoff time.Duration

	// MaxRetryBackoff is the maximum wait between retries.
	// Worst-case extra latency ≈ MaxRetries × MaxRetryBackoff.
	// Set to -1 to disable backoff cap.
	// Default: 512ms.
	MaxRetryBackoff time.Duration
}

// GlobalConnect creates a new Redis client from config, verifies it with
// PING, and stores it as the package-level client. It is safe to call
// concurrently (e.g. during a hot config reload).
func GlobalConnect(ctx context.Context, config Config) error {
	client, err := Connect(ctx, config)
	if err != nil {
		return err
	}

	SetClient(client)
	return nil
}

// Connect creates and validates a new Redis client from config.
// The returned client is independent of the package-level global —
// use GlobalConnect or SetClient to install it as the global.
func Connect(ctx context.Context, config Config) (redis.UniversalClient, error) {
	if ctx == nil {
		ctx = context.Background()
		logger.Warn(ctx, "Context is nil, using context.Background() in Redis Connect")
	}

	if len(config.Addrs) == 0 {
		return nil, errors.New("redisdb: at least one address is required in Config.Addrs")
	}

	applyDefaults(&config)

	// Validate pool hierarchy. go-redis silently clamps invalid values which
	// masks misconfiguration — fail explicitly instead.
	if config.MinIdleConns > config.MaxIdleConns {
		return nil, errors.New("redisdb: MinIdleConns must be ≤ MaxIdleConns")
	}
	if config.MaxIdleConns > config.PoolSize {
		return nil, errors.New("redisdb: MaxIdleConns must be ≤ PoolSize")
	}

	logger.Info(ctx, "Connecting to Redis")

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
		logger.Infof(ctx, "Using Redis Sentinel with master name: %s", config.MasterName)
	}

	// DB selection is only supported for standalone and Sentinel modes.
	// Cluster mode is auto-detected: multiple addresses with no MasterName.
	isClusterMode := len(config.Addrs) > 1 && config.MasterName == ""
	if !isClusterMode {
		options.DB = config.DB
		logger.Infof(ctx, "Using Redis DB: %d", config.DB)
	} else {
		logger.Info(ctx, "Using Redis Cluster mode (DB selection not supported, using DB 0)")
	}

	if config.TLSEnabled {
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		logger.Info(ctx, "TLS is enabled for Redis connection")
	}

	client := redis.NewUniversalClient(options)

	// Verify the connection is alive before returning.
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close() // release pool resources on failure
		logger.Errorw(ctx, "Failed to connect to Redis", "error", err)
		return nil, err
	}

	logger.Info(ctx, "Redis connection established successfully")
	return client, nil
}

// Close gracefully shuts down the package-level Redis client and releases
// all pool resources. Safe to call concurrently. Returns nil if no global
// client is set.
func Close() error {
	defaultMu.Lock()
	client := defaultClient
	defaultClient = nil
	defaultMu.Unlock()

	if client != nil {
		return client.Close()
	}
	return nil
}

// applyDefaults fills zero-value fields with production-ready defaults.
// Fields set to -1 by the caller (disable sentinel) are never overwritten.
func applyDefaults(config *Config) {
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
}
