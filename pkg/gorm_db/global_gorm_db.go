package gormdb

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
)

var defaultDBClient *DBClient

func init() {
	defaultDBClient = &DBClient{
		clientName: "globalDBClient",
	}
}

func GetDB(tx ...*gorm.DB) *gorm.DB {
	return defaultDBClient.GetDB(tx...)
}

func SetDB(database *gorm.DB) {
	defaultDBClient.SetDB(database)
}

func GetConfig() Config {
	return defaultDBClient.GetConfig()
}

func SetConfig(config Config) {
	defaultDBClient.SetConfig(config)
}

func GetClientName() string {
	return defaultDBClient.GetClientName()
}

func SetClientName(name string) {
	defaultDBClient.SetClientName(name)
}

func Connect(ctx context.Context) error {
	return defaultDBClient.Connect(ctx)
}

func Reconnect(ctx context.Context) error {
	return defaultDBClient.Reconnect(ctx)
}

func Close() error {
	return defaultDBClient.Close()
}

func BeginTransaction(opts ...*sql.TxOptions) *gorm.DB {
	return defaultDBClient.BeginTransaction(opts...)
}

func RollbackTransaction(tx *gorm.DB) *gorm.DB {
	return defaultDBClient.RollbackTransaction(tx)
}

func CommitTransaction(tx *gorm.DB) *gorm.DB {
	return defaultDBClient.CommitTransaction(tx)
}

func SavePoint(tx *gorm.DB, name string) *gorm.DB {
	return defaultDBClient.SavePoint(tx, name)
}

func RollbackToSavePoint(tx *gorm.DB, name string) *gorm.DB {
	return defaultDBClient.RollbackToSavePoint(tx, name)
}

func RunInTransaction(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error, opts ...*sql.TxOptions) error {
	return defaultDBClient.RunInTransaction(ctx, db, fn, opts...)
}

func ExecRawSQL(tx *gorm.DB, sql string, values ...any) *gorm.DB {
	return defaultDBClient.ExecRawSQL(tx, sql, values...)
}
