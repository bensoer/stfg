// Package bolt provides a stub BoltDB backend that returns ErrNotImplemented
// for all methods. It exists as a compile-time-checked placeholder for the
// future BoltDB implementation (Plan 3).
package bolt

import (
	"context"
	"errors"
	"time"

	"stfg/internal/storage"
)

var ErrNotImplemented = errors.New("bolt storage: not yet implemented")

type BoltStorage struct{}

func NewBolt(ctx context.Context) (*BoltStorage, error) {
	return &BoltStorage{}, ErrNotImplemented
}

func (b *BoltStorage) Close() error { return nil }

func (b *BoltStorage) Migrate(ctx context.Context) error { return ErrNotImplemented }

func (b *BoltStorage) AddGrocery(ctx context.Context, item storage.GroceryItem) error {
	return ErrNotImplemented
}

func (b *BoltStorage) RemoveGrocery(ctx context.Context, name string) error {
	return ErrNotImplemented
}

func (b *BoltStorage) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) {
	return nil, ErrNotImplemented
}

func (b *BoltStorage) HasGrocery(ctx context.Context, name string) (bool, error) {
	return false, ErrNotImplemented
}

func (b *BoltStorage) AddFlyer(ctx context.Context, flyer storage.Flyer) error {
	return ErrNotImplemented
}

func (b *BoltStorage) RemoveFlyer(ctx context.Context, id int64) error {
	return ErrNotImplemented
}

func (b *BoltStorage) GetFlyer(ctx context.Context, id int64) (*storage.Flyer, error) {
	return nil, ErrNotImplemented
}

func (b *BoltStorage) ListFlyers(ctx context.Context) ([]storage.Flyer, error) {
	return nil, ErrNotImplemented
}

func (b *BoltStorage) HasFlyer(ctx context.Context, id int64) (bool, error) {
	return false, ErrNotImplemented
}

func (b *BoltStorage) AddFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	return ErrNotImplemented
}

func (b *BoltStorage) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error {
	return ErrNotImplemented
}

func (b *BoltStorage) ListFlyerItems(ctx context.Context, flyerID int64) ([]storage.FlyerItem, error) {
	return nil, ErrNotImplemented
}

func (b *BoltStorage) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) {
	return false, ErrNotImplemented
}

func (b *BoltStorage) PruneExpired(ctx context.Context, now time.Time) error {
	return ErrNotImplemented
}

var _ storage.Storage = (*BoltStorage)(nil)