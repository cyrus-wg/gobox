package gormdb

import (
	"context"

	"gorm.io/gorm"
)

// Create inserts one or more records into the database.
// The value can be a pointer to a struct or a pointer to a slice of structs.
// It accepts optional *gorm.DB arguments to support transactions.
//
// Usage:
//
//	user := &User{Name: "Alice"}
//	err := dbc.Create(ctx, user)
//
//	// Batch insert:
//	users := &[]User{{Name: "Alice"}, {Name: "Bob"}}
//	err := dbc.Create(ctx, users)
//
//	// Within a transaction:
//	err := dbc.RunInTransaction(ctx, nil, func(tx *gorm.DB) error {
//	    return dbc.Create(ctx, &User{Name: "Bob"}, tx)
//	})
func (dbc *DBClient) Create(ctx context.Context, value any, tx ...*gorm.DB) error {
	return dbc.GetDB(tx...).WithContext(ctx).Create(value).Error
}
