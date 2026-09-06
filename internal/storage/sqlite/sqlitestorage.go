// Package sqlite provides a stub SQLite backend that returns ErrNotImplemented
// for all methods. It exists as a compile-time-checked placeholder for the
// future SQLite implementation (Plan 3).
package sqlite

import (
	"context"
	"errors"
	"time"

	"stfg/internal/storage"
)

var ErrNotImplemented = errors.New("sqlite storage: not yet implemented")

type SQLiteStorage struct{}

func NewSQLite(ctx context.Context) (*SQLiteStorage, error) {
	return &SQLiteStorage{}, ErrNotImplemented
}

func (s *SQLiteStorage) Close() error { return nil }

func (s *SQLiteStorage) Migrate(ctx context.Context) error { return ErrNotImplemented }

func (s *SQLiteStorage) AddGrocery(ctx context.Context, item storage.GroceryItem) error {
	return ErrNotImplemented
}

func (s *SQLiteStorage) RemoveGrocery(ctx context.Context, name string) error {
	return ErrNotImplemented
}

func (s *SQLiteStorage) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) {
	return nil, ErrNotImplemented
}

func (s *SQLiteStorage) HasGrocery(ctx context.Context, name string) (bool, error) {
	return false, ErrNotImplemented
}

func (s *SQLiteStorage) AddFlyer(ctx context.Context, flyer storage.Flyer) error {
	return ErrNotImplemented
}

func (s *SQLiteStorage) RemoveFlyer(ctx context.Context, id int64) error {
	return ErrNotImplemented
}

func (s *SQLiteStorage) GetFlyer(ctx context.Context, id int64) (*storage.Flyer, error) {
	return nil, ErrNotImplemented
}

func (s *SQLiteStorage) ListFlyers(ctx context.Context) ([]storage.Flyer, error) {
	return nil, ErrNotImplemented
}

func (s *SQLiteStorage) HasFlyer(ctx context.Context, id int64) (bool, error) {
	return false, ErrNotImplemented
}

func (s *SQLiteStorage) AddFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	return ErrNotImplemented
}

func (s *SQLiteStorage) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error {
	return ErrNotImplemented
}

func (s *SQLiteStorage) ListFlyerItems(ctx context.Context, flyerID int64) ([]storage.FlyerItem, error) {
	return nil, ErrNotImplemented
}

func (s *SQLiteStorage) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) {
	return false, ErrNotImplemented
}

func (s *SQLiteStorage) PruneExpired(ctx context.Context, now time.Time) error {
	return ErrNotImplemented
}

// Compile-time assertion that *SQLiteStorage satisfies storage.Storage.
// If *SQLiteStorage ever stops implementing the interface (e.g. a method
// signature changed), this line will produce a clear compile error.
var _ storage.Storage = (*SQLiteStorage)(nil)