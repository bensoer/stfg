package storage

import "errors"

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
