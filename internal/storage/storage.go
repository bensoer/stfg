package storage

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Sentinel errors. Backends wrap these with errors.Is-compatible errors
// so callers can use errors.Is to test for "not found", "duplicate", etc.
var (
	ErrNotFound  = errors.New("storage: not found")
	ErrDuplicate = errors.New("storage: duplicate")
	// ErrInvalidArgument is reserved for invalid-argument validation;
	// returned for empty/whitespace grocery names in AddGrocery, RemoveGrocery,
	// and HasGrocery across all three backends (sqlite, bolt, and JSON).
	ErrInvalidArgument = errors.New("storage: invalid argument")
)

// NormalizeName lowercases a grocery name for case-insensitive keying.
// Backends use this instead of strings.EqualFold so all three backends
// (JSON, sqlite, bolt) share the same normalization semantics. Under
// strings.EqualFold, certain Unicode pairs (e.g. "Σ"/"ς") are treated as
// duplicates, whereas strings.ToLower treats them as distinct — the shared
// ToLower semantics are what the parity contract pins.
func NormalizeName(name string) string {
	return strings.ToLower(name)
}

// Storage is the single contract every persistence backend must satisfy.
// All methods take a context.Context so backends can support cancellation/timeouts.
// Implementations must be safe for concurrent use within a single process.
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
