package json

import (
	"context"
	"errors"
	"time"

	"stfg/internal/storage"
)

var ErrNotImplemented = errors.New("json storage: not yet implemented")

type FileStorage struct{}

func NewJSON(ctx context.Context) (*FileStorage, error) {
	return &FileStorage{}, ErrNotImplemented
}

func (f *FileStorage) Close() error { return nil }

func (f *FileStorage) Migrate(ctx context.Context) error { return ErrNotImplemented }

func (f *FileStorage) AddGrocery(ctx context.Context, item storage.GroceryItem) error {
	return ErrNotImplemented
}

func (f *FileStorage) RemoveGrocery(ctx context.Context, name string) error {
	return ErrNotImplemented
}

func (f *FileStorage) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) {
	return nil, ErrNotImplemented
}

func (f *FileStorage) HasGrocery(ctx context.Context, name string) (bool, error) {
	return false, ErrNotImplemented
}

func (f *FileStorage) AddFlyer(ctx context.Context, flyer storage.Flyer) error {
	return ErrNotImplemented
}

func (f *FileStorage) RemoveFlyer(ctx context.Context, id int64) error {
	return ErrNotImplemented
}

func (f *FileStorage) GetFlyer(ctx context.Context, id int64) (*storage.Flyer, error) {
	return nil, ErrNotImplemented
}

func (f *FileStorage) ListFlyers(ctx context.Context) ([]storage.Flyer, error) {
	return nil, ErrNotImplemented
}

func (f *FileStorage) HasFlyer(ctx context.Context, id int64) (bool, error) {
	return false, ErrNotImplemented
}

func (f *FileStorage) AddFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	return ErrNotImplemented
}

func (f *FileStorage) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error {
	return ErrNotImplemented
}

func (f *FileStorage) ListFlyerItems(ctx context.Context, flyerID int64) ([]storage.FlyerItem, error) {
	return nil, ErrNotImplemented
}

func (f *FileStorage) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) {
	return false, ErrNotImplemented
}

func (f *FileStorage) PruneExpired(ctx context.Context, now time.Time) error {
	return ErrNotImplemented
}

var _ storage.Storage = (*FileStorage)(nil)