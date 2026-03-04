package main

import (
	"context"
	"fmt"
	"log"
	"time"

	gormdb "github.com/cyrus-wg/gobox/pkg/gorm_db"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ──────────────────────────────────────────────
// Model
// ──────────────────────────────────────────────

type Product struct {
	ID        uint           `gorm:"primaryKey"`
	Name      string         `gorm:"type:varchar(255);not null"`
	Price     float64        `gorm:"not null"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// ──────────────────────────────────────────────
// DSNs – must match docker-compose.yml
// ──────────────────────────────────────────────

const (
	postgresDSN = "host=localhost user=testuser password=testpass dbname=testdb port=5433 sslmode=disable TimeZone=UTC"
	mysqlDSN    = "testuser:testpass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=UTC"
)

// ──────────────────────────────────────────────
// Entry point
// ──────────────────────────────────────────────

func main() {
	ctx := context.Background()

	fmt.Println("========== PostgreSQL ==========")
	if err := runAllTests(ctx, gormdb.PostgreSQL, postgresDSN); err != nil {
		log.Fatalf("PostgreSQL tests failed: %v", err)
	}

	fmt.Println()
	fmt.Println("========== MySQL ==========")
	if err := runAllTests(ctx, gormdb.MySQL, mysqlDSN); err != nil {
		log.Fatalf("MySQL tests failed: %v", err)
	}

	fmt.Println()
	fmt.Println("All tests passed!")
}

// ──────────────────────────────────────────────
// runAllTests creates two clients to the same DB
// and runs every test group.
// ──────────────────────────────────────────────

func runAllTests(ctx context.Context, driver gormdb.Driver, dsn string) error {
	driverName := string(driver)

	// ── Create two clients pointing at the same database ──
	clientA := gormdb.NewDBClient(driverName+"-clientA", gormdb.Config{
		Driver:       driver,
		DSN:          dsn,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	})
	clientB := gormdb.NewDBClient(driverName+"-clientB", gormdb.Config{
		Driver:       driver,
		DSN:          dsn,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	})

	if err := clientA.Connect(ctx); err != nil {
		return fmt.Errorf("connect clientA: %w", err)
	}
	defer clientA.Close()

	if err := clientB.Connect(ctx); err != nil {
		return fmt.Errorf("connect clientB: %w", err)
	}
	defer clientB.Close()

	// ── Prepare schema ──
	fmt.Println("[setup] Dropping existing Product table (if any) …")
	if err := clientA.GetDB().Migrator().DropTable(&Product{}); err != nil {
		return fmt.Errorf("drop table: %w", err)
	}
	fmt.Println("[setup] AutoMigrate Product table …")
	if err := clientA.GetDB().AutoMigrate(&Product{}); err != nil {
		return fmt.Errorf("auto-migrate: %w", err)
	}

	// ── Seed data ──
	fmt.Println("[setup] Inserting seed data …")
	seeds := []Product{
		{Name: "Widget", Price: 9.99},
		{Name: "Gadget", Price: 24.50},
		{Name: "Doohickey", Price: 3.75},
	}
	if err := clientA.GetDB().Create(&seeds).Error; err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	// ── Test groups ──
	tests := []struct {
		name string
		fn   func(ctx context.Context, a, b *gormdb.DBClient) error
	}{
		{"Getters & Setters", testGettersSetters},
		{"Connect with empty DSN", testConnectEmptyDSN},
		{"Connect with nil context", testConnectNilContext},
		{"Connect with unsupported driver", testConnectUnsupportedDriver},
		{"applyDefaults preserves custom values", testApplyDefaultsCustomValues},
		{"Custom GormConfig preserved", testCustomGormConfig},
		{"NewGormLogger & LogMode", testGormLoggerAPI},
		{"SetConfig re-applies defaults", testSetConfigDefaults},
		{"CRUD", testCRUD},
		{"Transaction commit (two-client)", testTxCommitTwoClients},
		{"Transaction rollback (two-client)", testTxRollbackTwoClients},
		{"RunInTransaction (two-client)", testRunInTransaction},
		{"SavePoint & RollbackToSavePoint (two-client)", testSavePoint},
		{"ExecRawSQL (two-client)", testRawSQL},
		{"Monitoring", testMonitoring},
		{"Reconnect with monitoring", testReconnectWithMonitoring},
		{"Reconnect", testReconnect},
		{"Close idempotency", testCloseIdempotent},
		{"Global functions", testGlobalFunctions},
	}

	for _, tt := range tests {
		fmt.Printf("[test]  %s\n", tt.name)
		if err := tt.fn(ctx, clientA, clientB); err != nil {
			return fmt.Errorf("%s: %w", tt.name, err)
		}
	}

	fmt.Printf("[ok]    All checks passed for %s\n", driverName)
	return nil
}

// ──────────────────────────────────────────────
// Getters & Setters
// ──────────────────────────────────────────────

func testGettersSetters(_ context.Context, a, _ *gormdb.DBClient) error {
	// GetClientName / SetClientName
	origName := a.GetClientName()
	a.SetClientName("renamed-client")
	assertEq("SetClientName", "renamed-client", a.GetClientName())
	a.SetClientName(origName) // restore

	// GetConfig / SetConfig
	origCfg := a.GetConfig()
	newCfg := origCfg
	newCfg.MaxOpenConns = 42
	a.SetConfig(newCfg)
	assertEq("SetConfig MaxOpenConns", 42, a.GetConfig().MaxOpenConns)
	a.SetConfig(origCfg) // restore

	// GetDB / SetDB
	origDB := a.GetDB()
	assertNeq("GetDB not nil", nil, origDB)
	a.SetDB(origDB) // no-op round-trip

	// GetDB with tx override
	tx := a.GetDB().Begin()
	got := a.GetDB(tx)
	assertEq("GetDB(tx) returns tx", true, got == tx)
	tx.Rollback()

	// GetDB with nil tx should fall back to the stored db
	got = a.GetDB(nil)
	assertEq("GetDB(nil) returns db", true, got == origDB)

	// GetMonitoringEnabled default
	assertEq("default monitoring off", false, a.GetMonitoringEnabled())

	// GetMonitoringLogInterval / SetMonitoringLogInterval
	a.SetMonitoringLogInterval(5 * time.Second)
	assertEq("SetMonitoringLogInterval", 5*time.Second, a.GetMonitoringLogInterval())

	// SetMonitoringLogInterval with too-small value should be rejected
	a.SetMonitoringLogInterval(500 * time.Millisecond) // <= 1s, ignored
	assertEq("interval unchanged after bad value", 5*time.Second, a.GetMonitoringLogInterval())

	return nil
}

// ──────────────────────────────────────────────
// Connect with empty DSN → must return error
// ──────────────────────────────────────────────

func testConnectEmptyDSN(_ context.Context, _, _ *gormdb.DBClient) error {
	bad := gormdb.NewDBClient("bad", gormdb.Config{Driver: gormdb.PostgreSQL, DSN: ""})
	err := bad.Connect(context.Background())
	if err == nil {
		return fmt.Errorf("expected error for empty DSN, got nil")
	}
	fmt.Printf("        ✓ empty DSN returned error: %s\n", err)
	return nil
}

// ──────────────────────────────────────────────
// Connect with nil context → exercises the
// nil-ctx fallback path (should not panic).
// ──────────────────────────────────────────────

func testConnectNilContext(_ context.Context, a, _ *gormdb.DBClient) error {
	driver := a.GetConfig().Driver
	dsn := a.GetConfig().DSN

	tmp := gormdb.NewDBClient("nil-ctx", gormdb.Config{Driver: driver, DSN: dsn})
	//nolint:staticcheck // intentionally passing nil context
	if err := tmp.Connect(nil); err != nil {
		return fmt.Errorf("connect with nil ctx: %w", err)
	}
	defer tmp.Close()

	// Verify the connection is usable.
	var count int64
	if err := tmp.GetDB().Model(&Product{}).Count(&count).Error; err != nil {
		return fmt.Errorf("query after nil-ctx connect: %w", err)
	}
	fmt.Printf("        ✓ nil-ctx connect succeeded, row count: %d\n", count)
	return nil
}

// ──────────────────────────────────────────────
// Connect with unsupported driver → must return
// a dialector error.
// ──────────────────────────────────────────────

func testConnectUnsupportedDriver(_ context.Context, _, _ *gormdb.DBClient) error {
	bad := gormdb.NewDBClient("bad-driver", gormdb.Config{
		Driver: gormdb.Driver("sqlite"),
		DSN:    "file::memory:",
	})
	err := bad.Connect(context.Background())
	if err == nil {
		return fmt.Errorf("expected error for unsupported driver, got nil")
	}
	fmt.Printf("        ✓ unsupported driver returned error: %s\n", err)
	return nil
}

// ──────────────────────────────────────────────
// applyDefaults preserves custom values: when
// the caller provides explicit pool/logger
// settings, they must not be overwritten.
// ──────────────────────────────────────────────

func testApplyDefaultsCustomValues(_ context.Context, a, _ *gormdb.DBClient) error {
	driver := a.GetConfig().Driver
	dsn := a.GetConfig().DSN

	ignFalse := false
	custom := gormdb.Config{
		Driver:                    driver,
		DSN:                       dsn,
		MaxOpenConns:              99,
		MaxIdleConns:              77,
		ConnMaxIdleTime:           1 * time.Minute,
		ConnMaxLifetime:           2 * time.Minute,
		SlowQueryThreshold:        500 * time.Millisecond,
		IgnoreRecordNotFoundError: &ignFalse,
		GormLogLevel:              gormlogger.Info,
	}

	tmp := gormdb.NewDBClient("custom-defaults", custom)
	cfg := tmp.GetConfig()

	assertEq("custom MaxOpenConns", 99, cfg.MaxOpenConns)
	assertEq("custom MaxIdleConns", 77, cfg.MaxIdleConns)
	assertEq("custom ConnMaxIdleTime", 1*time.Minute, cfg.ConnMaxIdleTime)
	assertEq("custom ConnMaxLifetime", 2*time.Minute, cfg.ConnMaxLifetime)
	assertEq("custom SlowQueryThreshold", 500*time.Millisecond, cfg.SlowQueryThreshold)
	assertEq("custom IgnoreRecordNotFound", false, *cfg.IgnoreRecordNotFoundError)
	assertEq("custom GormLogLevel", gormlogger.Info, cfg.GormLogLevel)

	// GormConfig should have been created by applyDefaults.
	assertNeq("GormConfig not nil", (*gorm.Config)(nil), cfg.GormConfig)

	return nil
}

// ──────────────────────────────────────────────
// Custom GormConfig: when the caller supplies
// their own *gorm.Config (with or without a
// logger), it must be preserved as-is.
// ──────────────────────────────────────────────

func testCustomGormConfig(_ context.Context, a, _ *gormdb.DBClient) error {
	driver := a.GetConfig().Driver
	dsn := a.GetConfig().DSN

	customLogger := gormdb.NewGormLogger()
	customCfg := &gorm.Config{Logger: customLogger}

	tmp := gormdb.NewDBClient("custom-gormcfg", gormdb.Config{
		Driver:     driver,
		DSN:        dsn,
		GormConfig: customCfg,
	})
	resolvedCfg := tmp.GetConfig()

	// The pointer should be preserved.
	assertEq("GormConfig pointer preserved", true, resolvedCfg.GormConfig == customCfg)
	assertEq("custom logger preserved", true, resolvedCfg.GormConfig.Logger == customLogger)

	// Also verify Connect works with the custom config.
	if err := tmp.Connect(context.Background()); err != nil {
		return fmt.Errorf("connect with custom GormConfig: %w", err)
	}
	defer tmp.Close()

	var count int64
	if err := tmp.GetDB().Model(&Product{}).Count(&count).Error; err != nil {
		return fmt.Errorf("query after custom GormConfig connect: %w", err)
	}
	fmt.Printf("        ✓ custom GormConfig connect succeeded, row count: %d\n", count)
	return nil
}

// ──────────────────────────────────────────────
// NewGormLogger & LogMode: exercise the
// GormLogger constructor and LogMode method.
// ──────────────────────────────────────────────

func testGormLoggerAPI(_ context.Context, _, _ *gormdb.DBClient) error {
	lg := gormdb.NewGormLogger()

	// Defaults from NewGormLogger.
	assertEq("default SlowThreshold", 200*time.Millisecond, lg.SlowThreshold)
	assertEq("default LogLevel", gormlogger.Warn, lg.LogLevel)
	assertEq("default IgnoreRecordNotFound", true, lg.IgnoreRecordNotFoundError)

	// LogMode returns a new logger with the given level.
	silent := lg.LogMode(gormlogger.Silent)
	assertNeq("LogMode returns non-nil", nil, silent)

	// The original should be unchanged (LogMode returns a clone).
	assertEq("original level unchanged", gormlogger.Warn, lg.LogLevel)

	// The new logger should have the requested level.
	silentGL, ok := silent.(*gormdb.GormLogger)
	assertEq("LogMode type assertion", true, ok)
	assertEq("LogMode level", gormlogger.Silent, silentGL.LogLevel)

	fmt.Println("        ✓ NewGormLogger / LogMode work correctly")
	return nil
}

// ──────────────────────────────────────────────
// SetConfig re-applies defaults: calling
// SetConfig with zero-value fields should fill
// in the defaults.
// ──────────────────────────────────────────────

func testSetConfigDefaults(_ context.Context, a, _ *gormdb.DBClient) error {
	driver := a.GetConfig().Driver
	dsn := a.GetConfig().DSN

	tmp := gormdb.NewDBClient("setconfig-test", gormdb.Config{
		Driver:       driver,
		DSN:          dsn,
		MaxOpenConns: 50,
	})

	// SetConfig with only Driver/DSN — all pool fields should get defaults.
	tmp.SetConfig(gormdb.Config{
		Driver: driver,
		DSN:    dsn,
	})
	cfg := tmp.GetConfig()

	assertEq("SetConfig MaxOpenConns default", 25, cfg.MaxOpenConns)
	assertEq("SetConfig MaxIdleConns default", 10, cfg.MaxIdleConns)
	assertEq("SetConfig SlowQuery default", 200*time.Millisecond, cfg.SlowQueryThreshold)
	assertEq("SetConfig GormLogLevel default", gormlogger.Warn, cfg.GormLogLevel)
	assertNeq("SetConfig GormConfig not nil", (*gorm.Config)(nil), cfg.GormConfig)

	fmt.Println("        ✓ SetConfig re-applies defaults correctly")
	return nil
}

// ──────────────────────────────────────────────
// CRUD (uses clientA only — basic operations)
// ──────────────────────────────────────────────

func testCRUD(_ context.Context, a, _ *gormdb.DBClient) error {
	db := a.GetDB()

	// Read all.
	var products []Product
	if err := db.Find(&products).Error; err != nil {
		return fmt.Errorf("find all: %w", err)
	}
	assertEq("seed row count", 3, len(products))

	// Read one.
	var widget Product
	if err := db.Where("name = ?", "Widget").First(&widget).Error; err != nil {
		return fmt.Errorf("find widget: %w", err)
	}
	assertEq("widget price", 9.99, widget.Price)

	// Update.
	if err := db.Model(&widget).Update("price", 12.99).Error; err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if err := db.First(&widget, widget.ID).Error; err != nil {
		return fmt.Errorf("re-read: %w", err)
	}
	assertEq("updated price", 12.99, widget.Price)

	// Soft-delete.
	if err := db.Delete(&widget).Error; err != nil {
		return fmt.Errorf("soft-delete: %w", err)
	}
	var remaining []Product
	if err := db.Find(&remaining).Error; err != nil {
		return fmt.Errorf("find after delete: %w", err)
	}
	assertEq("rows after soft-delete", 2, len(remaining))

	// Unscoped.
	var all []Product
	if err := db.Unscoped().Find(&all).Error; err != nil {
		return fmt.Errorf("unscoped: %w", err)
	}
	assertEq("unscoped count", 3, len(all))

	return nil
}

// ──────────────────────────────────────────────
// Transaction commit (two-client):
// clientA writes in TX; clientB verifies the row
// is NOT visible before commit, then IS visible
// after commit.
// ──────────────────────────────────────────────

func testTxCommitTwoClients(_ context.Context, a, b *gormdb.DBClient) error {
	tx := a.BeginTransaction()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Create(&Product{Name: "TxCommitItem", Price: 5.00}).Error; err != nil {
		a.RollbackTransaction(tx)
		return err
	}

	// Before commit: clientB should NOT see the row.
	var countBefore int64
	b.GetDB().Model(&Product{}).Where("name = ?", "TxCommitItem").Count(&countBefore)
	assertEq("before commit, clientB sees 0", int64(0), countBefore)

	a.CommitTransaction(tx)

	// After commit: clientB should see the row.
	var p Product
	if err := b.GetDB().Where("name = ?", "TxCommitItem").First(&p).Error; err != nil {
		return fmt.Errorf("committed row not visible to clientB: %w", err)
	}
	assertEq("tx commit price via clientB", 5.00, p.Price)

	return nil
}

// ──────────────────────────────────────────────
// Transaction rollback (two-client):
// clientA writes in TX then rolls back; clientB
// confirms the row never appears.
// ──────────────────────────────────────────────

func testTxRollbackTwoClients(_ context.Context, a, b *gormdb.DBClient) error {
	tx := a.BeginTransaction()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Create(&Product{Name: "TxRollbackItem", Price: 7.00}).Error; err != nil {
		a.RollbackTransaction(tx)
		return err
	}

	a.RollbackTransaction(tx)

	// clientB checks: row should NOT exist.
	var count int64
	b.GetDB().Model(&Product{}).Where("name = ?", "TxRollbackItem").Count(&count)
	assertEq("rolled-back row absent via clientB", int64(0), count)

	return nil
}

// ──────────────────────────────────────────────
// RunInTransaction (two-client)
// ──────────────────────────────────────────────

func testRunInTransaction(ctx context.Context, a, b *gormdb.DBClient) error {
	// Successful transaction.
	err := a.RunInTransaction(ctx, nil, func(tx *gorm.DB) error {
		return tx.Create(&Product{Name: "RunInTxItem", Price: 11.11}).Error
	})
	if err != nil {
		return err
	}

	// Verify via clientB.
	var p Product
	if err := b.GetDB().Where("name = ?", "RunInTxItem").First(&p).Error; err != nil {
		return fmt.Errorf("RunInTransaction row not visible to clientB: %w", err)
	}
	assertEq("RunInTransaction price via clientB", 11.11, p.Price)

	// Failed transaction: returning an error should rollback.
	_ = a.RunInTransaction(ctx, nil, func(tx *gorm.DB) error {
		tx.Create(&Product{Name: "RunInTxFail", Price: 0.01})
		return fmt.Errorf("intentional error")
	})

	var failCount int64
	b.GetDB().Model(&Product{}).Where("name = ?", "RunInTxFail").Count(&failCount)
	assertEq("RunInTransaction error rollback via clientB", int64(0), failCount)

	return nil
}

// ──────────────────────────────────────────────
// SavePoint & RollbackToSavePoint (two-client)
// ──────────────────────────────────────────────

func testSavePoint(_ context.Context, a, b *gormdb.DBClient) error {
	tx := a.BeginTransaction()
	if tx.Error != nil {
		return tx.Error
	}

	if err := tx.Create(&Product{Name: "BeforeSP", Price: 1.00}).Error; err != nil {
		a.RollbackTransaction(tx)
		return err
	}

	a.SavePoint(tx, "sp1")

	if err := tx.Create(&Product{Name: "AfterSP", Price: 2.00}).Error; err != nil {
		a.RollbackTransaction(tx)
		return err
	}

	a.RollbackToSavePoint(tx, "sp1")
	a.CommitTransaction(tx)

	// clientB verifies.
	var beforeCount, afterCount int64
	b.GetDB().Model(&Product{}).Where("name = ?", "BeforeSP").Count(&beforeCount)
	b.GetDB().Model(&Product{}).Where("name = ?", "AfterSP").Count(&afterCount)
	assertEq("BeforeSP via clientB", int64(1), beforeCount)
	assertEq("AfterSP rolled back via clientB", int64(0), afterCount)

	return nil
}

// ──────────────────────────────────────────────
// ExecRawSQL (two-client)
// ──────────────────────────────────────────────

func testRawSQL(_ context.Context, a, b *gormdb.DBClient) error {
	tx := a.BeginTransaction()
	if tx.Error != nil {
		return tx.Error
	}

	result := a.ExecRawSQL(tx, "UPDATE products SET price = price + 1 WHERE name = ?", "Gadget")
	if result.Error != nil {
		a.RollbackTransaction(tx)
		return result.Error
	}
	a.CommitTransaction(tx)

	// Verify via clientB.
	var gadget Product
	if err := b.GetDB().Where("name = ?", "Gadget").First(&gadget).Error; err != nil {
		return fmt.Errorf("find Gadget via clientB: %w", err)
	}
	assertEq("Gadget price after raw +1 via clientB", 25.50, gadget.Price)

	return nil
}

// ──────────────────────────────────────────────
// Monitoring: enable with a short interval, sleep
// so at least one tick fires, then disable and
// verify everything shuts down cleanly.
// ──────────────────────────────────────────────

func testMonitoring(_ context.Context, a, _ *gormdb.DBClient) error {
	// Set a short interval so we don't wait long.
	a.SetMonitoringLogInterval(2 * time.Second)
	assertEq("interval set", 2*time.Second, a.GetMonitoringLogInterval())

	// Enable monitoring.
	a.SetMonitoringEnabled(true)
	assertEq("monitoring on", true, a.GetMonitoringEnabled())

	// Idempotent: setting the same value again should not panic/deadlock.
	a.SetMonitoringEnabled(true)
	assertEq("monitoring still on", true, a.GetMonitoringEnabled())

	// Wait for at least one monitoring tick to fire (check logs in output).
	fmt.Println("        … sleeping 3s to let monitoring tick …")
	time.Sleep(3 * time.Second)

	// Disable monitoring.
	a.SetMonitoringEnabled(false)
	assertEq("monitoring off", false, a.GetMonitoringEnabled())

	// Idempotent disable.
	a.SetMonitoringEnabled(false)
	assertEq("monitoring still off", false, a.GetMonitoringEnabled())

	fmt.Println("        ✓ monitoring enabled/disabled without deadlock")
	return nil
}

// ──────────────────────────────────────────────
// Reconnect with monitoring: enable monitoring,
// then reconnect. Monitoring should survive the
// reconnect cycle (Close stops it, Connect
// restarts it).
// ──────────────────────────────────────────────

func testReconnectWithMonitoring(ctx context.Context, a, _ *gormdb.DBClient) error {
	a.SetMonitoringLogInterval(2 * time.Second)
	a.SetMonitoringEnabled(true)
	assertEq("monitoring on before reconnect", true, a.GetMonitoringEnabled())

	if err := a.Reconnect(ctx); err != nil {
		return fmt.Errorf("reconnect with monitoring: %w", err)
	}

	// After Reconnect, monitoring should still be enabled.
	assertEq("monitoring on after reconnect", true, a.GetMonitoringEnabled())

	// Verify the reconnected DB is functional.
	var count int64
	if err := a.GetDB().Unscoped().Model(&Product{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count after reconnect with monitoring: %w", err)
	}
	fmt.Printf("        ✓ reconnect with monitoring OK, row count: %d\n", count)

	// Clean up.
	a.SetMonitoringEnabled(false)
	return nil
}

// ──────────────────────────────────────────────
// Reconnect
// ──────────────────────────────────────────────

func testReconnect(ctx context.Context, a, _ *gormdb.DBClient) error {
	if err := a.Reconnect(ctx); err != nil {
		return fmt.Errorf("reconnect: %w", err)
	}

	var count int64
	if err := a.GetDB().Unscoped().Model(&Product{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count after reconnect: %w", err)
	}
	fmt.Printf("        ✓ row count after reconnect (unscoped): %d\n", count)
	return nil
}

// ──────────────────────────────────────────────
// Close idempotency: calling Close twice must not
// panic or return an error the second time.
// ──────────────────────────────────────────────

func testCloseIdempotent(ctx context.Context, a, _ *gormdb.DBClient) error {
	driver := a.GetConfig().Driver
	dsn := a.GetConfig().DSN

	tmp := gormdb.NewDBClient("close-test", gormdb.Config{
		Driver: driver,
		DSN:    dsn,
	})
	if err := tmp.Connect(ctx); err != nil {
		return fmt.Errorf("connect tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("first close: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("second close should be nil: %w", err)
	}
	fmt.Println("        ✓ double Close returned nil")
	return nil
}

// ──────────────────────────────────────────────
// Global functions: exercise every package-level
// wrapper that delegates to the default DBClient.
// ──────────────────────────────────────────────

func testGlobalFunctions(ctx context.Context, a, _ *gormdb.DBClient) error {
	driver := a.GetConfig().Driver
	dsn := a.GetConfig().DSN

	// SetConfig + Connect via global helpers.
	gormdb.SetConfig(gormdb.Config{
		Driver: driver,
		DSN:    dsn,
	})
	assertEq("global GetConfig driver", driver, gormdb.GetConfig().Driver)

	if err := gormdb.Connect(ctx); err != nil {
		return fmt.Errorf("global Connect: %w", err)
	}
	defer gormdb.Close()

	// GetClientName / SetClientName.
	gormdb.SetClientName("global-test")
	assertEq("global client name", "global-test", gormdb.GetClientName())

	// GetDB / SetDB.
	gdb := gormdb.GetDB()
	assertNeq("global GetDB not nil", nil, gdb)
	gormdb.SetDB(gdb) // round-trip

	// Monitoring setters via global.
	gormdb.SetMonitoringLogInterval(5 * time.Second)
	assertEq("global interval", 5*time.Second, gormdb.GetMonitoringLogInterval())
	assertEq("global monitoring off", false, gormdb.GetMonitoringEnabled())
	gormdb.SetMonitoringEnabled(true)
	assertEq("global monitoring on", true, gormdb.GetMonitoringEnabled())
	gormdb.SetMonitoringEnabled(false)

	// BeginTransaction + CommitTransaction via global.
	tx := gormdb.BeginTransaction()
	if tx.Error != nil {
		return tx.Error
	}
	tx.Create(&Product{Name: "GlobalTxItem", Price: 99.99})
	gormdb.CommitTransaction(tx)

	var p Product
	if err := gormdb.GetDB().Where("name = ?", "GlobalTxItem").First(&p).Error; err != nil {
		return fmt.Errorf("global tx committed row: %w", err)
	}
	assertEq("global tx price", 99.99, p.Price)

	// RollbackTransaction via global.
	tx2 := gormdb.BeginTransaction()
	if tx2.Error != nil {
		return tx2.Error
	}
	tx2.Create(&Product{Name: "GlobalRollback", Price: 0.01})
	gormdb.RollbackTransaction(tx2)

	var rollbackCount int64
	gormdb.GetDB().Model(&Product{}).Where("name = ?", "GlobalRollback").Count(&rollbackCount)
	assertEq("global rollback absent", int64(0), rollbackCount)

	// SavePoint / RollbackToSavePoint via global.
	tx3 := gormdb.BeginTransaction()
	if tx3.Error != nil {
		return tx3.Error
	}
	tx3.Create(&Product{Name: "GlobalSPBefore", Price: 1.00})
	gormdb.SavePoint(tx3, "gsp1")
	tx3.Create(&Product{Name: "GlobalSPAfter", Price: 2.00})
	gormdb.RollbackToSavePoint(tx3, "gsp1")
	gormdb.CommitTransaction(tx3)

	var spBefore, spAfter int64
	gormdb.GetDB().Model(&Product{}).Where("name = ?", "GlobalSPBefore").Count(&spBefore)
	gormdb.GetDB().Model(&Product{}).Where("name = ?", "GlobalSPAfter").Count(&spAfter)
	assertEq("global SP before", int64(1), spBefore)
	assertEq("global SP after rollback", int64(0), spAfter)

	// RunInTransaction via global.
	err := gormdb.RunInTransaction(ctx, nil, func(tx *gorm.DB) error {
		return tx.Create(&Product{Name: "GlobalRunInTx", Price: 77.77}).Error
	})
	if err != nil {
		return fmt.Errorf("global RunInTransaction: %w", err)
	}
	var rtx Product
	if err := gormdb.GetDB().Where("name = ?", "GlobalRunInTx").First(&rtx).Error; err != nil {
		return fmt.Errorf("global RunInTransaction row: %w", err)
	}
	assertEq("global RunInTransaction price", 77.77, rtx.Price)

	// ExecRawSQL via global.
	tx4 := gormdb.BeginTransaction()
	if tx4.Error != nil {
		return tx4.Error
	}
	gormdb.ExecRawSQL(tx4, "UPDATE products SET price = 0 WHERE name = ?", "GlobalRunInTx")
	gormdb.CommitTransaction(tx4)

	var updated Product
	gormdb.GetDB().Where("name = ?", "GlobalRunInTx").First(&updated)
	assertEq("global ExecRawSQL price", 0.00, updated.Price)

	// Reconnect via global.
	if err := gormdb.Reconnect(ctx); err != nil {
		return fmt.Errorf("global Reconnect: %w", err)
	}
	var afterReconnect int64
	gormdb.GetDB().Unscoped().Model(&Product{}).Count(&afterReconnect)
	fmt.Printf("        ✓ global row count after reconnect: %d\n", afterReconnect)

	fmt.Println("        ✓ all global function wrappers exercised")
	return nil
}

// ──────────────────────────────────────────────
// Assertion helpers
// ──────────────────────────────────────────────

func assertEq[T comparable](label string, want, got T) {
	if want != got {
		panic(fmt.Sprintf("FAIL %s: want %v, got %v", label, want, got))
	}
	fmt.Printf("        ✓ %s == %v\n", label, got)
}

func assertNeq[T comparable](label string, notWant, got T) {
	if notWant == got {
		panic(fmt.Sprintf("FAIL %s: did not want %v", label, notWant))
	}
	fmt.Printf("        ✓ %s != %v\n", label, notWant)
}
