package redisdb

import (
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps a redis.UniversalClient with connection management,
// config storage, and thread-safe accessors. Use NewRedisClient to create
// one, or use the package-level functions that operate on a default global.
type RedisClient struct {
	client      redis.UniversalClient
	mu          sync.RWMutex // protects client, clientName, and redisConfig
	clientName  string
	redisConfig Config
}

// NewRedisClient creates a new RedisClient with the given name and config.
// Call Connect on the returned client to establish the connection.
//
//	rc := redisdb.NewRedisClient("orders-cache", redisdb.Config{
//	    Addrs: []string{"localhost:6379"},
//	})
//	if err := rc.Connect(ctx); err != nil { ... }
func NewRedisClient(name string, config Config) *RedisClient {
	return &RedisClient{
		clientName:  name,
		redisConfig: config,
	}
}

// BlockingConfig returns a copy of base pre-tuned for blocking commands such
// as BLPOP, BRPOP, BZPOPMIN, XREAD BLOCK, and SUBSCRIBE/PSUBSCRIBE.
//
// Changes from standard defaults:
//   - ReadTimeout = -1 (disabled) — the server holds the connection open until
//     data arrives; a read deadline would fire prematurely.
//   - MaxRetries  = -1 (disabled) — retrying a blocking command resets the
//     block, which is almost never the desired behaviour.
//
// Always bound blocking calls with a context deadline:
//
//	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
//	defer cancel()
//	val, err := client.BLPop(ctx, 0, "mylist").Result()
func BlockingConfig(base Config) Config {
	base.ReadTimeout = -1
	base.MaxRetries = -1
	return base
}

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
