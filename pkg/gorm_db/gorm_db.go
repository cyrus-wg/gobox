package gormdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
)

func (dbc *DBClient) GetDB(tx ...*gorm.DB) *gorm.DB {
	if len(tx) > 0 && tx[0] != nil {
		return tx[0]
	}
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.db
}

func (dbc *DBClient) SetDB(database *gorm.DB) {
	dbc.mu.Lock()
	dbc.db = database
	dbc.mu.Unlock()
}

func (dbc *DBClient) GetConfig() Config {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.dbConfig
}

func (dbc *DBClient) SetConfig(config Config) {
	dbc.mu.Lock()
	dbc.dbConfig = config
	dbc.mu.Unlock()
}

func (dbc *DBClient) GetClientName() string {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.clientName
}

func (dbc *DBClient) SetClientName(name string) {
	dbc.mu.Lock()
	logger.Infow(context.Background(), "Setting database client name", "oldName", dbc.clientName, "newName", name)
	dbc.clientName = name
	dbc.mu.Unlock()
}

func (dbc *DBClient) Connect(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
		logger.Warnw(ctx, "Context is nil, using context.Background() in database Connect", "clientName", dbc.GetClientName())
	}

	config := applyDefaults(dbc.GetConfig())
	dbc.SetConfig(config)

	if config.DSN == "" {
		return errors.New("gormdb: DSN is required")
	}

	// Build the GORM config.
	gormCfg := config.GormConfig
	if gormCfg == nil {
		gormCfg = &gorm.Config{}
	}

	// Open the primary (writer) connection.
	dialector, err := newDialector(config.Driver, config.DSN)
	if err != nil {
		logger.Errorw(ctx, "Failed to create GORM dialector", "error", err, "driver", config.Driver, "clientName", dbc.GetClientName())
		return fmt.Errorf("gormdb: failed to create dialector: %w", err)
	}

	logger.Infow(ctx, "Connecting to database", "driver", config.Driver, "clientName", dbc.GetClientName())

	database, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		logger.Errorw(ctx, "Failed to open database connection", "error", err, "driver", config.Driver, "clientName", dbc.GetClientName())
		return fmt.Errorf("gormdb: failed to open database: %w", err)
	}

	dbc.SetDB(database)

	// Configure the underlying sql.DB connection pool.
	if err := dbc.configureSQLDB(); err != nil {
		logger.Errorw(ctx, "Failed to configure SQL DB connection pool", "error", err, "clientName", dbc.GetClientName())
		return fmt.Errorf("gormdb: failed to configure SQL DB: %w", err)
	}

	// Register read replicas via dbresolver if any are provided.
	if len(config.Replica.DSN) > 0 {
		if err := dbc.registerReplicas(); err != nil {
			logger.Errorw(ctx, "Failed to register read replicas", "error", err, "clientName", dbc.GetClientName())
			return fmt.Errorf("gormdb: failed to register replicas: %w", err)
		}
		logger.Infow(ctx, "Read replicas registered successfully", "replicaCount", len(config.Replica.DSN), "clientName", dbc.GetClientName())
	}

	// Verify connectivity.
	sqlDB, err := dbc.GetDB().DB()
	if err != nil {
		logger.Errorw(ctx, "Failed to obtain *sql.DB for connectivity check", "error", err, "clientName", dbc.GetClientName())
		return fmt.Errorf("gormdb: cannot obtain *sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		dbc.Close() // close the pool if ping fails
		logger.Errorw(ctx, "Database PING failed", "error", err, "clientName", dbc.GetClientName())
		return fmt.Errorf("gormdb: ping failed: %w", err)
	}

	logger.Infow(ctx, "Database connection established successfully", "clientName", dbc.GetClientName())
	return nil
}

func (dbc *DBClient) Reconnect(ctx context.Context) error {
	logger.Infow(ctx, "Reconnecting to database", "clientName", dbc.GetClientName())
	if err := dbc.Close(); err != nil {
		logger.Errorw(ctx, "Failed to close existing database connection during reconnect", "error", err, "clientName", dbc.GetClientName())
		// Continue with reconnect attempt even if close fails, as the existing connection may be in a bad state.
	}

	return dbc.Connect(ctx)
}

func (dbc *DBClient) Close() error {
	dbc.mu.Lock()
	database := dbc.db
	dbc.db = nil
	dbc.mu.Unlock()

	if database == nil {
		return nil
	}
	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("gormdb: cannot obtain *sql.DB for close: %w", err)
	}
	return sqlDB.Close()
}

func (dbc *DBClient) BeginTransaction(opts ...*sql.TxOptions) *gorm.DB {
	return dbc.GetDB().Begin(opts...)
}

func (dbc *DBClient) RollbackTransaction(tx *gorm.DB) *gorm.DB {
	return tx.Rollback()
}

func (dbc *DBClient) CommitTransaction(tx *gorm.DB) *gorm.DB {
	return tx.Commit()
}

func (dbc *DBClient) SavePoint(tx *gorm.DB, name string) *gorm.DB {
	return tx.SavePoint(name)
}

func (dbc *DBClient) RollbackToSavePoint(tx *gorm.DB, name string) *gorm.DB {
	return tx.RollbackTo(name)
}

func (dbc *DBClient) RunInTransaction(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions) error {
	return dbc.GetDB(db).Transaction(fn, opts...)
}

func (dbc *DBClient) ExecRawSQL(tx *gorm.DB, sql string, values ...any) *gorm.DB {
	return tx.Exec(sql, values...)
}

func (dbc *DBClient) configureSQLDB() error {
	sqlDB, err := dbc.GetDB().DB()
	if err != nil {
		return fmt.Errorf("gormdb: cannot obtain *sql.DB: %w", err)
	}

	config := dbc.GetConfig()
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	return nil
}

func (dbc *DBClient) registerReplicas() error {
	config := dbc.GetConfig()
	replicas := make([]gorm.Dialector, 0, len(config.Replica.DSN))
	for _, dsn := range config.Replica.DSN {
		d, err := newDialector(config.Driver, dsn)
		if err != nil {
			return err
		}
		replicas = append(replicas, d)
	}

	resolver := dbresolver.Register(dbresolver.Config{
		Replicas: replicas,
		Policy:   dbresolver.RandomPolicy{},
	}).
		SetMaxOpenConns(config.MaxOpenConns).
		SetMaxIdleConns(config.MaxIdleConns).
		SetConnMaxIdleTime(config.ConnMaxIdleTime).
		SetConnMaxLifetime(config.ConnMaxLifetime)

	return dbc.db.Use(resolver)
}

func (dbc *DBClient) monitorConnectionState() {
	ctx := context.Background()
	ticker := time.NewTicker(dbc.GetConfig().MonitoringLogInterval)
	defer ticker.Stop()

	for range ticker.C {
		db := dbc.GetDB()

		if db == nil {
			logger.Warnw(ctx, "Database client is nil during connection monitoring", "clientName", dbc.GetClientName())
			continue
		}

		sqlDB, err := db.DB()
		if err != nil {
			logger.Warnw(ctx, "Failed to obtain *sql.DB for health check", "error", err, "clientName", dbc.GetClientName())
			continue
		}

		stats := sqlDB.Stats()
		logger.Infow(ctx, "Database connection pool stats",
			"clientName", dbc.GetClientName(),
			"maxOpenConns", stats.MaxOpenConnections,
			"openConns", stats.OpenConnections,
			"inUse", stats.InUse,
			"idle", stats.Idle,
			"waitCount", stats.WaitCount,
			"waitDurationInSeconds", stats.WaitDuration.Seconds(),
		)
	}
}

func applyDefaults(config Config) Config {
	if config.MaxOpenConns <= 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxIdleConns <= 0 {
		config.MaxIdleConns = 10
	}
	if config.ConnMaxIdleTime <= 0 {
		config.ConnMaxIdleTime = 5 * time.Minute
	}
	if config.ConnMaxLifetime <= 0 {
		config.ConnMaxLifetime = 30 * time.Minute
	}

	if len(config.Replica.DSN) > 0 {
		if config.Replica.MaxOpenConns <= 0 {
			config.Replica.MaxOpenConns = 25
		}
		if config.Replica.MaxIdleConns <= 0 {
			config.Replica.MaxIdleConns = 10
		}
		if config.Replica.ConnMaxIdleTime <= 0 {
			config.Replica.ConnMaxIdleTime = 5 * time.Minute
		}
		if config.Replica.ConnMaxLifetime <= 0 {
			config.Replica.ConnMaxLifetime = 30 * time.Minute
		}
	}

	if config.MonitoringEnabled && config.MonitoringLogInterval < 10*time.Second {
		config.MonitoringLogInterval = 30 * time.Second
	}

	return config
}

func newDialector(driver Driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case MySQL:
		return mysql.Open(dsn), nil
	case PostgreSQL:
		return postgres.Open(dsn), nil
	default:
		return nil, fmt.Errorf("gormdb: unsupported driver %q (supported: mysql, postgres)", driver)
	}
}
