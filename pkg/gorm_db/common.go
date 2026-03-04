package gormdb

import (
	"sync"
	"time"

	"gorm.io/gorm"
)

// Driver identifies the database engine.
type Driver string

const (
	MySQL      Driver = "mysql"
	PostgreSQL Driver = "postgres"
)

// DBClient wraps a *gorm.DB with connection management, read-replica
// support, pool configuration, and optional connection monitoring.
// Use NewDBClient to create one, or use the package-level functions
// that operate on a default global instance.
type DBClient struct {
	db         *gorm.DB
	mu         sync.RWMutex // protects db, clientName, dbConfig
	clientName string
	dbConfig   Config

	monitoringEnabled     bool
	monitoringLogInterval time.Duration
	stopMonitor           func() // cancels the monitoring goroutine's context; nil when not running
}

// NewDBClient creates a new DBClient with the given name and config.
// Call Connect on the returned client to establish the connection.
//
//	dbc := gormdb.NewDBClient("orders-db", gormdb.Config{
//	    Driver: gormdb.PostgreSQL,
//	    DSN:    "host=localhost user=app dbname=orders ...",
//	})
//	if err := dbc.Connect(ctx); err != nil { ... }
func NewDBClient(name string, config Config) *DBClient {
	return &DBClient{
		clientName: name,
		dbConfig:   config,
	}
}

// Config holds everything needed to open a primary database connection,
// optionally register read replicas, and tune the connection pool.
type Config struct {
	// Driver selects the SQL dialect (MySQL or PostgreSQL).
	Driver Driver

	// DSN is the primary (writer) data-source name. Required.
	DSN string

	// Connection pool tuning for the primary connection.
	// Zero values are replaced with safe defaults by applyDefaults.
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration

	// Replica holds optional read-replica DSNs and their own pool settings.
	Replica ReplicaConfig

	// GormConfig is forwarded to gorm.Open. If nil, a default &gorm.Config{}
	// is used.
	GormConfig *gorm.Config
}

// ReplicaConfig holds DSNs and pool settings for read replicas.
// All replicas share the same Driver as the primary connection.
type ReplicaConfig struct {
	DSN             []string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}
