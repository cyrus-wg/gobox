package gormdb

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite" // pure-Go SQLite driver (no cgo)
	"gorm.io/gorm"
)

// widget is a simple model used by the Repository tests.
type widget struct {
	ID    uint `gorm:"primaryKey"`
	Name  string
	Price int
}

// dbSeq gives every test its own uniquely-named shared-cache in-memory DB so
// tests remain fully isolated from one another.
var dbSeq atomic.Int64

// newTestDB opens a fresh in-memory SQLite database with the widget table
// migrated. Each call gets a distinct database name.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("file:gormtest%d?mode=memory&cache=shared", dbSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&widget{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newTestClient returns a *DBClient backed by an isolated in-memory SQLite
// database. It bypasses Connect (which only knows mysql / postgres) by
// injecting the *gorm.DB directly.
func newTestClient(t *testing.T) *DBClient {
	t.Helper()
	dbc := NewDBClient("test", Config{})
	dbc.SetDB(newTestDB(t))
	return dbc
}

func TestRepository_CreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))

	w := &widget{Name: "gadget", Price: 10}
	if err := repo.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}
	if w.ID == 0 {
		t.Fatal("expected ID to be populated after create")
	}

	got, err := repo.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "gadget" || got.Price != 10 {
		t.Fatalf("unexpected record: %+v", got)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))

	_, err := repo.GetByID(ctx, 9999)
	if !IsNotFound(err) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestRepository_CreateManyAndFind(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))

	if err := repo.CreateMany(ctx, []widget{
		{Name: "a", Price: 1},
		{Name: "b", Price: 2},
		{Name: "c", Price: 3},
	}); err != nil {
		t.Fatalf("create many: %v", err)
	}

	all, err := repo.Find(ctx, Query{Order: "price asc"})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(all) != 3 || all[0].Name != "a" || all[2].Name != "c" {
		t.Fatalf("unexpected list: %+v", all)
	}

	// CreateMany with empty slice is a no-op.
	if err := repo.CreateMany(ctx, nil); err != nil {
		t.Fatalf("create many empty: %v", err)
	}
}

func TestRepository_FindWithFilterAndPagination(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))
	_ = repo.CreateMany(ctx, []widget{
		{Name: "x", Price: 5},
		{Name: "y", Price: 15},
		{Name: "z", Price: 25},
	})

	got, err := repo.Find(ctx, Query{
		Where: "price >= ?",
		Args:  []any{10},
		Order: "price asc",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got) != 1 || got[0].Name != "y" {
		t.Fatalf("unexpected filtered list: %+v", got)
	}
}

func TestRepository_FirstAndCountAndExists(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))
	_ = repo.CreateMany(ctx, []widget{
		{Name: "dup", Price: 1},
		{Name: "dup", Price: 2},
		{Name: "solo", Price: 3},
	})

	first, err := repo.First(ctx, Query{Where: map[string]any{"name": "dup"}, Order: "price asc"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Price != 1 {
		t.Fatalf("expected price 1, got %d", first.Price)
	}

	count, err := repo.Count(ctx, Query{Where: "name = ?", Args: []any{"dup"}})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}

	exists, err := repo.Exists(ctx, Query{Where: "name = ?", Args: []any{"solo"}})
	if err != nil || !exists {
		t.Fatalf("expected exists, got %v err=%v", exists, err)
	}
	missing, _ := repo.Exists(ctx, Query{Where: "name = ?", Args: []any{"nope"}})
	if missing {
		t.Fatal("expected not exists")
	}
}

func TestRepository_UpdateByID(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))
	w := &widget{Name: "old", Price: 1}
	_ = repo.Create(ctx, w)

	n, err := repo.UpdateByID(ctx, w.ID, map[string]any{"name": "new", "price": 99})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row affected, got %d", n)
	}

	got, _ := repo.GetByID(ctx, w.ID)
	if got.Name != "new" || got.Price != 99 {
		t.Fatalf("update not applied: %+v", got)
	}
}

func TestRepository_Update_Filtered(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))
	_ = repo.CreateMany(ctx, []widget{
		{Name: "keep", Price: 1},
		{Name: "bump", Price: 2},
		{Name: "bump", Price: 3},
	})

	n, err := repo.Update(ctx, Query{Where: "name = ?", Args: []any{"bump"}}, map[string]any{"price": 100})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows, got %d", n)
	}
}

func TestRepository_DeleteByIDAndDelete(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))
	_ = repo.CreateMany(ctx, []widget{
		{Name: "d1", Price: 1},
		{Name: "d2", Price: 2},
		{Name: "keep", Price: 3},
	})

	first, _ := repo.First(ctx, Query{Where: "name = ?", Args: []any{"d1"}})
	n, err := repo.DeleteByID(ctx, first.ID)
	if err != nil || n != 1 {
		t.Fatalf("delete by id: n=%d err=%v", n, err)
	}

	n, err = repo.Delete(ctx, Query{Where: "name = ?", Args: []any{"d2"}})
	if err != nil || n != 1 {
		t.Fatalf("delete filtered: n=%d err=%v", n, err)
	}

	// Delete without a Where must be rejected.
	if _, err := repo.Delete(ctx, Query{}); err == nil {
		t.Fatal("expected error deleting with no filter")
	}

	count, _ := repo.Count(ctx, Query{})
	if count != 1 {
		t.Fatalf("expected 1 remaining, got %d", count)
	}
}

func TestRepository_DeleteAll(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))
	_ = repo.CreateMany(ctx, []widget{{Name: "a"}, {Name: "b"}})

	n, err := repo.DeleteAll(ctx)
	if err != nil {
		t.Fatalf("delete all: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}
	count, _ := repo.Count(ctx, Query{})
	if count != 0 {
		t.Fatalf("expected empty table, got %d", count)
	}
}

func TestRepository_Transaction_Commit(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))

	err := repo.Transaction(ctx, func(tx *Repository[widget]) error {
		if err := tx.Create(ctx, &widget{Name: "tx1", Price: 1}); err != nil {
			return err
		}
		return tx.Create(ctx, &widget{Name: "tx2", Price: 2})
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}

	count, _ := repo.Count(ctx, Query{})
	if count != 2 {
		t.Fatalf("expected 2 committed rows, got %d", count)
	}
}

func TestRepository_Transaction_Rollback(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository[widget](newTestClient(t))

	sentinel := context.DeadlineExceeded
	err := repo.Transaction(ctx, func(tx *Repository[widget]) error {
		if err := tx.Create(ctx, &widget{Name: "rollback", Price: 1}); err != nil {
			return err
		}
		return sentinel // force rollback
	})
	if err != sentinel {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	count, _ := repo.Count(ctx, Query{})
	if count != 0 {
		t.Fatalf("expected rollback (0 rows), got %d", count)
	}
}

func TestRepo_GlobalBinding(t *testing.T) {
	// Repo[T]() binds to the global default client; wire it to an in-memory DB.
	SetDB(newTestDB(t))

	ctx := context.Background()
	repo := Repo[widget]()
	if err := repo.Create(ctx, &widget{Name: "global", Price: 7}); err != nil {
		t.Fatalf("create via global repo: %v", err)
	}
	got, err := repo.First(ctx, Query{Where: "name = ?", Args: []any{"global"}})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.Price != 7 {
		t.Fatalf("expected 7, got %d", got.Price)
	}
}
