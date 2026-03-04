package gormdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
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
	dbc.dbConfig = applyDefaults(config)
	dbc.mu.Unlock()
}

func (dbc *DBClient) GetClientName() string {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.clientName
}

func (dbc *DBClient) SetClientName(name string) {
	dbc.mu.Lock()
	oldName := dbc.clientName
	dbc.clientName = name
	dbc.mu.Unlock()

	logger.Infow(context.Background(), "Setting database client name", "oldName", oldName, "newName", name)
}

func (dbc *DBClient) GetMonitoringEnabled() bool {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.monitoringEnabled
}

func (dbc *DBClient) SetMonitoringEnabled(enabled bool) {
	dbc.mu.Lock()
	if dbc.monitoringEnabled == enabled {
		dbc.mu.Unlock()
		return
	}

	dbc.monitoringEnabled = enabled

	// Stop any existing monitor goroutine.
	if dbc.stopMonitor != nil {
		dbc.stopMonitor()
		dbc.stopMonitor = nil
	}

	if enabled {
		if dbc.monitoringLogInterval <= 0 {
			dbc.monitoringLogInterval = 30 * time.Second
		}

		monitorCtx, cancel := context.WithCancel(context.Background())
		dbc.stopMonitor = cancel
		clientName := dbc.clientName
		dbc.mu.Unlock()

		go dbc.monitorConnectionState(monitorCtx)
		logger.Infow(context.Background(), "Database connection monitoring enabled", "clientName", clientName)
	} else {
		clientName := dbc.clientName
		dbc.mu.Unlock()

		logger.Infow(context.Background(), "Database connection monitoring disabled", "clientName", clientName)
	}
}

func (dbc *DBClient) GetMonitoringLogInterval() time.Duration {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.monitoringLogInterval
}

func (dbc *DBClient) SetMonitoringLogInterval(interval time.Duration) {
	if interval <= time.Second {
		logger.Warnw(context.Background(), "Monitoring log interval must be greater than 1 second", "providedInterval", interval, "clientName", dbc.clientName)
		return
	}
	dbc.mu.Lock()
	dbc.monitoringLogInterval = interval
	dbc.mu.Unlock()
}

func (dbc *DBClient) Connect(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
		logger.Warnw(ctx, "Context is nil, using context.Background() in database Connect", "clientName", dbc.clientName)
	}

	// Ensure defaults are applied and take a lock-safe local snapshot.
	dbc.SetConfig(dbc.GetConfig())
	config := dbc.GetConfig()

	if config.DSN == "" {
		return errors.New("gormdb: DSN is required")
	}

	// Open the primary (writer) connection.
	dialector, err := newDialector(config.Driver, config.DSN)
	if err != nil {
		logger.Errorw(ctx, "Failed to create GORM dialector", "error", err, "driver", config.Driver, "clientName", dbc.clientName)
		return fmt.Errorf("gormdb: failed to create dialector: %w", err)
	}

	logger.Infow(ctx, "Connecting to database", "driver", config.Driver, "clientName", dbc.clientName)

	database, err := gorm.Open(dialector, config.GormConfig)
	if err != nil {
		logger.Errorw(ctx, "Failed to open database connection", "error", err, "driver", config.Driver, "clientName", dbc.clientName)
		return fmt.Errorf("gormdb: failed to open database: %w", err)
	}

	dbc.SetDB(database)

	// Configure the underlying sql.DB connection pool.
	if err := dbc.configureSQLDB(); err != nil {
		logger.Errorw(ctx, "Failed to configure SQL DB connection pool", "error", err, "clientName", dbc.clientName)
		return fmt.Errorf("gormdb: failed to configure SQL DB: %w", err)
	}

	// Register read replicas via dbresolver if any are provided.
	if len(config.Replica.DSN) > 0 {
		if err := dbc.registerReplicas(); err != nil {
			logger.Errorw(ctx, "Failed to register read replicas", "error", err, "clientName", dbc.clientName)
			return fmt.Errorf("gormdb: failed to register replicas: %w", err)
		}
		logger.Infow(ctx, "Read replicas registered successfully", "replicaCount", len(config.Replica.DSN), "clientName", dbc.clientName)
	}

	// Verify connectivity.
	sqlDB, err := dbc.GetDB().DB()
	if err != nil {
		logger.Errorw(ctx, "Failed to obtain *sql.DB for connectivity check", "error", err, "clientName", dbc.clientName)
		return fmt.Errorf("gormdb: cannot obtain *sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		dbc.Close() // close the pool if ping fails
		logger.Errorw(ctx, "Database PING failed", "error", err, "clientName", dbc.clientName)
		return fmt.Errorf("gormdb: ping failed: %w", err)
	}

	// Start monitoring connection state if enabled.
	dbc.mu.Lock()
	if dbc.monitoringEnabled {
		if dbc.stopMonitor != nil {
			dbc.stopMonitor()
		}
		monitorCtx, cancel := context.WithCancel(context.Background())
		dbc.stopMonitor = cancel
		go dbc.monitorConnectionState(monitorCtx)
	}
	dbc.mu.Unlock()

	logger.Infow(ctx, "Database connection established successfully", "clientName", dbc.clientName)
	return nil
}

func (dbc *DBClient) Reconnect(ctx context.Context) error {
	logger.Infow(ctx, "Reconnecting to database", "clientName", dbc.clientName)
	if err := dbc.Close(); err != nil {
		logger.Errorw(ctx, "Failed to close existing database connection during reconnect", "error", err, "clientName", dbc.clientName)
		// Continue with reconnect attempt even if close fails, as the existing connection may be in a bad state.
	}

	return dbc.Connect(ctx)
}

func (dbc *DBClient) Close() error {
	dbc.mu.Lock()
	database := dbc.db
	dbc.db = nil
	if dbc.stopMonitor != nil {
		dbc.stopMonitor()
		dbc.stopMonitor = nil
	}
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
		SetMaxOpenConns(config.Replica.MaxOpenConns).
		SetMaxIdleConns(config.Replica.MaxIdleConns).
		SetConnMaxIdleTime(config.Replica.ConnMaxIdleTime).
		SetConnMaxLifetime(config.Replica.ConnMaxLifetime)

	return dbc.GetDB().Use(resolver)
}

func (dbc *DBClient) monitorConnectionState(ctx context.Context) {
	ticker := time.NewTicker(dbc.GetMonitoringLogInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			db := dbc.GetDB()
			if db == nil {
				logger.Warnw(ctx, "Database client is nil during connection monitoring", "clientName", dbc.clientName)
				continue
			}

			sqlDB, err := db.DB()
			if err != nil {
				logger.Warnw(ctx, "Failed to obtain *sql.DB for health check", "error", err, "clientName", dbc.clientName)
				continue
			}

			stats := sqlDB.Stats()
			logger.Infow(ctx, "Database connection pool stats",
				"clientName", dbc.clientName,
				"maxOpenConns", stats.MaxOpenConnections,
				"openConns", stats.OpenConnections,
				"inUse", stats.InUse,
				"idle", stats.Idle,
				"waitCount", stats.WaitCount,
				"waitDurationInSeconds", stats.WaitDuration.Seconds(),
			)
		}
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

	// Logger defaults.
	if config.SlowQueryThreshold == 0 {
		config.SlowQueryThreshold = 200 * time.Millisecond
	}
	if config.IgnoreRecordNotFoundError == nil {
		t := true
		config.IgnoreRecordNotFoundError = &t
	}
	if config.GormLogLevel == 0 {
		config.GormLogLevel = gormlogger.Warn
	}

	// Ensure GormConfig is initialized.
	if config.GormConfig == nil {
		config.GormConfig = &gorm.Config{}
	}

	// Use the structured JSON logger when no custom logger has been set.
	if config.GormConfig.Logger == nil {
		config.GormConfig.Logger = &GormLogger{
			SlowThreshold:             config.SlowQueryThreshold,
			IgnoreRecordNotFoundError: *config.IgnoreRecordNotFoundError,
			LogLevel:                  config.GormLogLevel,
		}
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
