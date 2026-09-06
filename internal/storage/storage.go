package storage

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors. Backends wrap these with errors.Is-compatible errors
// so callers can use errors.Is to test for "not found", "duplicate", etc.
var (
	ErrNotFound        = errors.New("storage: not found")
	ErrDuplicate       = errors.New("storage: duplicate")
	ErrInvalidArgument = errors.New("storage: invalid argument")
)

// Storage is the single contract every persistence backend must satisfy.
// All methods take a context.Context so backends can support cancellation/timeouts.
type Storage interface {
	// Lifecycle
	Close() error
	// Migrate creates any required schema/buckets/tables if missing. Idempotent.
	Migrate(ctx context.Context) error

	// Groceries
	AddGrocery(ctx context.Context, item GroceryItem) error
	RemoveGrocery(ctx context.Context, name string) error // case-insensitive match
	ListGroceries(ctx context.Context) ([]GroceryItem, error)
	HasGrocery(ctx context.Context, name string) (bool, error) // case-insensitive

	// Flyers (retail groups)
	AddFlyer(ctx context.Context, flyer Flyer) error
	RemoveFlyer(ctx context.Context, id int64) error
	GetFlyer(ctx context.Context, id int64) (*Flyer, error)
	ListFlyers(ctx context.Context) ([]Flyer, error)
	HasFlyer(ctx context.Context, id int64) (bool, error)

	// Flyer items
	AddFlyerItem(ctx context.Context, item FlyerItem) error
	RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error
	ListFlyerItems(ctx context.Context, flyerID int64) ([]FlyerItem, error)
	HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error)

	// Maintenance
	// PruneExpired removes flyers whose ValidTo is before now, and cascades
	// to their items. Idempotent.
	PruneExpired(ctx context.Context, now time.Time) error
}