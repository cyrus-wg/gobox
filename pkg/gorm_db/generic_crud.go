package gormdb

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Create inserts one or more records. value may be a pointer to a struct or a
// pointer to a slice of structs. An optional *gorm.DB (e.g. a transaction) can
// be supplied to run within an existing transaction.
//
//	user := &User{Name: "Alice"}
//	err := dbc.Create(ctx, user)
//
//	users := &[]User{{Name: "Alice"}, {Name: "Bob"}} // batch insert
//	err := dbc.Create(ctx, users)
func (dbc *DBClient) Create(ctx context.Context, value any, tx ...*gorm.DB) error {
	return dbc.GetDB(tx...).WithContext(ctx).Create(value).Error
}

// ---------------------------------------------------------------------------
// Repository[T] — a generic, type-safe CRUD helper.
// ---------------------------------------------------------------------------

// Repository provides common, type-safe CRUD operations for a model type T.
// T must be a struct type mapped by GORM. Create one with NewRepository (bound
// to a *DBClient) or Repo (bound to the global client).
//
//	type User struct { ID uint; Name string }
//	users := gormdb.NewRepository[User](dbc)
//	u, err := users.GetByID(ctx, 1)
//	list, err := users.List(ctx, gormdb.Query{Limit: 20, Order: "id desc"})
type Repository[T any] struct {
	client *DBClient
	// bound, when non-nil, is used as the base *gorm.DB for every operation.
	// It is set for repositories created inside Transaction so that all calls
	// run within the same transaction.
	bound *gorm.DB
}

// NewRepository returns a Repository[T] bound to the given client.
func NewRepository[T any](client *DBClient) *Repository[T] {
	return &Repository[T]{client: client}
}

// Repo returns a Repository[T] bound to the global default client.
func Repo[T any]() *Repository[T] {
	return &Repository[T]{client: defaultDBClient}
}

// Query describes optional filtering, ordering, pagination, preloading, and
// projection applied by read methods (Find, List, Count, First).
type Query struct {
	// Where is a GORM condition. It may be a string with Args placeholders,
	// a struct, or a map. Nil means no filter.
	Where any
	// Args are the placeholder arguments for a string Where clause.
	Args []any
	// Order is an ORDER BY clause, e.g. "created_at desc".
	Order string
	// Limit caps the number of rows (<= 0 means no limit).
	Limit int
	// Offset skips the first N rows (<= 0 means none).
	Offset int
	// Preloads lists association names to eager-load.
	Preloads []string
	// Select restricts the columns returned.
	Select []string
}

// apply builds a *gorm.DB with the query options applied.
func (q Query) apply(db *gorm.DB) *gorm.DB {
	if len(q.Select) > 0 {
		db = db.Select(q.Select)
	}
	if q.Where != nil {
		db = db.Where(q.Where, q.Args...)
	}
	for _, p := range q.Preloads {
		db = db.Preload(p)
	}
	if q.Order != "" {
		db = db.Order(q.Order)
	}
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	if q.Offset > 0 {
		db = db.Offset(q.Offset)
	}
	return db
}

func (r *Repository[T]) db(ctx context.Context, tx []*gorm.DB) *gorm.DB {
	// Explicit per-call tx wins, then a transaction-bound handle, then the
	// client's current DB.
	if len(tx) > 0 && tx[0] != nil {
		return tx[0].WithContext(ctx)
	}
	if r.bound != nil {
		return r.bound.WithContext(ctx)
	}
	return r.client.GetDB().WithContext(ctx)
}

// Create inserts a single record.
func (r *Repository[T]) Create(ctx context.Context, model *T, tx ...*gorm.DB) error {
	return r.db(ctx, tx).Create(model).Error
}

// CreateMany inserts a batch of records in a single statement.
func (r *Repository[T]) CreateMany(ctx context.Context, models []T, tx ...*gorm.DB) error {
	if len(models) == 0 {
		return nil
	}
	return r.db(ctx, tx).Create(&models).Error
}

// GetByID fetches a record by primary key. It returns ErrRecordNotFound when no
// row matches.
func (r *Repository[T]) GetByID(ctx context.Context, id any, tx ...*gorm.DB) (*T, error) {
	var model T
	err := r.db(ctx, tx).First(&model, id).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// First returns the first record matching q, or ErrRecordNotFound if none.
func (r *Repository[T]) First(ctx context.Context, q Query, tx ...*gorm.DB) (*T, error) {
	var model T
	err := q.apply(r.db(ctx, tx)).First(&model).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// Find returns all records matching q (an empty slice when none match).
func (r *Repository[T]) Find(ctx context.Context, q Query, tx ...*gorm.DB) ([]T, error) {
	var models []T
	err := q.apply(r.db(ctx, tx)).Find(&models).Error
	if err != nil {
		return nil, err
	}
	return models, nil
}

// List is an alias for Find, provided for readability at call sites.
func (r *Repository[T]) List(ctx context.Context, q Query, tx ...*gorm.DB) ([]T, error) {
	return r.Find(ctx, q, tx...)
}

// Count returns the number of rows matching q's Where filter. Order, Limit,
// Offset, Preloads, and Select are ignored.
func (r *Repository[T]) Count(ctx context.Context, q Query, tx ...*gorm.DB) (int64, error) {
	var count int64
	var model T
	db := r.db(ctx, tx).Model(&model)
	if q.Where != nil {
		db = db.Where(q.Where, q.Args...)
	}
	err := db.Count(&count).Error
	return count, err
}

// Exists reports whether at least one row matches q's Where filter.
func (r *Repository[T]) Exists(ctx context.Context, q Query, tx ...*gorm.DB) (bool, error) {
	count, err := r.Count(ctx, Query{Where: q.Where, Args: q.Args}, tx...)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save upserts a full record (all fields) by primary key.
func (r *Repository[T]) Save(ctx context.Context, model *T, tx ...*gorm.DB) error {
	return r.db(ctx, tx).Save(model).Error
}

// UpdateByID applies a partial update (map of column→value, or a struct) to the
// record with the given primary key. It returns the number of rows affected;
// zero rows with a nil error means no matching record.
func (r *Repository[T]) UpdateByID(ctx context.Context, id any, values any, tx ...*gorm.DB) (int64, error) {
	var model T
	res := r.db(ctx, tx).Model(&model).Where("id = ?", id).Updates(values)
	return res.RowsAffected, res.Error
}

// Update applies a partial update to all rows matching q's Where filter,
// returning the number of rows affected. A nil Where updates every row, which
// GORM blocks by default unless a global-update condition is set — pass an
// explicit Where to be safe.
func (r *Repository[T]) Update(ctx context.Context, q Query, values any, tx ...*gorm.DB) (int64, error) {
	var model T
	db := r.db(ctx, tx).Model(&model)
	if q.Where != nil {
		db = db.Where(q.Where, q.Args...)
	}
	res := db.Updates(values)
	return res.RowsAffected, res.Error
}

// DeleteByID deletes the record with the given primary key, returning the
// number of rows affected. When the model has a gorm.DeletedAt field this is a
// soft delete.
func (r *Repository[T]) DeleteByID(ctx context.Context, id any, tx ...*gorm.DB) (int64, error) {
	var model T
	res := r.db(ctx, tx).Delete(&model, id)
	return res.RowsAffected, res.Error
}

// Delete deletes all rows matching q's Where filter, returning the number of
// rows affected. A nil Where is rejected to avoid accidentally deleting every
// row; use DeleteAll for that.
func (r *Repository[T]) Delete(ctx context.Context, q Query, tx ...*gorm.DB) (int64, error) {
	if q.Where == nil {
		return 0, errors.New("gormdb: Delete requires a Where filter; use DeleteAll to remove every row")
	}
	var model T
	res := r.db(ctx, tx).Where(q.Where, q.Args...).Delete(&model)
	return res.RowsAffected, res.Error
}

// DeleteAll removes every row of T. It exists as an explicit, hard-to-misuse
// counterpart to Delete.
func (r *Repository[T]) DeleteAll(ctx context.Context, tx ...*gorm.DB) (int64, error) {
	var model T
	res := r.db(ctx, tx).Where("1 = 1").Delete(&model)
	return res.RowsAffected, res.Error
}

// Transaction runs fn inside a database transaction. The txRepo passed to fn
// executes every operation within that transaction, so callers don't need to
// thread a *gorm.DB manually:
//
//	err := users.Transaction(ctx, func(txRepo *gormdb.Repository[User]) error {
//	    if err := txRepo.Create(ctx, &u); err != nil { return err }
//	    _, err := txRepo.UpdateByID(ctx, other.ID, map[string]any{"active": true})
//	    return err
//	})
func (r *Repository[T]) Transaction(ctx context.Context, fn func(txRepo *Repository[T]) error) error {
	return r.client.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repository[T]{client: r.client, bound: tx})
	})
}

// IsNotFound reports whether err is (or wraps) ErrRecordNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}
