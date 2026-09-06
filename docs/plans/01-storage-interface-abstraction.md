# Plan 1 — Standardised Storage Abstraction

**Order:** 1 of 3
**Status:** Draft — DO NOT EXECUTE
**Depends on:** nothing
**Blocks:** Plans 2 and 3 (both implementations must conform to this contract)

## Goal

Define a single, backend-agnostic `Storage` interface in `internal/storage` that captures every CRUD action the JSON layer currently performs. After this plan lands, every caller in `cmd/`, `internal/reconciler/`, and `internal/promptwriter/` should depend only on the interface — never on a concrete implementation. Plans 2 and 3 then provide concrete implementations that satisfy the interface.

## Why this comes first

Both current implementations (`db.go` package-level functions and `JSONFileStorage` struct methods) disagree on schemas, filenames, error styles, and even the seed type for the empty `flyers_index.json`. Without a contract first, any amalgamated JSON implementation will simply bake the current mess in. Plan 1 fixes the contract. Plans 2 and 3 are swaps under that contract.

## Ben-mcp-server / context7 / grepp guidance

- **ben-mcp-server:** no relevant standards (the published standards cover Python, Git workflows, and skill authoring only). Proceeding with Go-idiomatic conventions.
- **context7 / grepp:** not consulted — these are design choices internal to a single Go module. No external library decision is being made here.

## Scope

### In scope

- One `Storage` interface in a new file `internal/storage/storage.go`.
- One canonical set of types (the "storage" records) in `internal/storage/types.go` — replacing the duplicate type definitions that today exist in `db.go`'s `types.go` and in `internal/reconciler/scrape/types.go`.
- One error contract: sentinel errors + the rule that "not found" is always returned as a sentinel, never as a wrapped generic error.
- A `Config` / `Options` struct for opening a backend (path, file mode, driver name).
- A factory `Open(opts Options) (Storage, error)` that selects a backend by name.

### Out of scope

- Any concrete implementation work — that is Plan 2 (JSON) and Plan 3 (SQLite, BoltDB).
- Removing the existing `db.go` and `jsonfilestorage.go` — that is Plan 2.
- Changing how callers (Cobra commands, reconciler) are wired — Plan 2 wires them through the interface once it exists.

## Proposed design

### File layout after this plan

```
internal/storage/
├── storage.go        # Storage interface, Options, sentinel errors, factory
├── types.go          # canonical GroceryItem, Flyer, FlyerItem, Store, *Item IDs
├── json/             # Plan 2 lands here
│   └── jsonfilestorage.go
├── sqlite/           # Plan 3 lands here
│   └── sqlitestorage.go
└── bolt/             # Plan 3 lands here
    └── boltstorage.go
```

### The `Storage` interface

```go
// internal/storage/storage.go
package storage

import (
    "context"
    "io"
)

type Backend string

const (
    BackendJSON   Backend = "json"
    BackendSQLite Backend = "sqlite"
    BackendBolt   Backend = "bolt"
)

type Options struct {
    // Backend selects the driver. Required.
    Backend Backend

    // CacheDir is the directory the backend should use. Required.
    // All backends write here; filename conventions are private to each backend.
    CacheDir string

    // FileMode is the mode applied to files/dirs created by the backend.
    // Defaults to 0o600 (files) / 0o700 (dirs).
    FileMode os.FileMode

    // JSON-specific (ignored by other backends)
    Indent bool // pretty-print JSON; default true for json backend

    // SQLite-specific
    // SQLiteFileName string // optional override; default "stfg.sqlite"

    // BoltDB-specific
    // BoltFileName string // optional override; default "stfg.bolt"
}

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

// Open returns the backend described by opts. The caller must call Close.
func Open(ctx context.Context, opts Options) (Storage, error) { ... }
```

### Canonical types (in `types.go`)

The current codebase has **three** competing type sets:

| Concept | `internal/storage/types.go` | `internal/reconciler/scrape/types.go` | Current on-disk field names |
|---|---|---|---|
| Grocery | `GroceryItem{Name, Embedding}` | (none) | `name`, `embedding` |
| Flyer | `Flyer{...}` | `RetailGroup{...}` | snake_case vs camelCase |
| Flyer item | `FlyerItem{...}` | `RetailGroupItem{...}` | snake_case vs camelCase |
| Store location | `Store{...}` | `RetailGroupLocation{...}` | `postal_code` vs `postalCode` |

Plan 1 chooses **one** canonical set, named with the everyday vocabulary (`Grocery`, `Flyer`, `FlyerItem`, `Store`) — not `RetailGroup` — and uses **snake_case JSON tags** consistently. This matches what the CLI commands and the promptwriter already import (`storage.GroceryItem`, `storage.FlyerItem`). The reconciler's `RetailGroup*` types then disappear; the reconciler imports `storage.Flyer`/`storage.FlyerItem` instead.

```go
// internal/storage/types.go
package storage

// GroceryItem is one item on the user's shopping list with its embedding.
type GroceryItem struct {
    Name      string    `json:"name"`
    Embedding []float32 `json:"embedding,omitempty"`
}

// Flyer is one retailer's flyer / retail group.
type Flyer struct {
    ID        int64     `json:"id"`
    ValidFrom time.Time `json:"valid_from"`
    ValidTo   time.Time `json:"valid_to"`
    Name      string    `json:"name"`
    Merchant  string    `json:"merchant"`
    Stores    []Store   `json:"stores,omitempty"`
}

// FlyerItem is one item inside a flyer.
type FlyerItem struct {
    ID        int64   `json:"id"`
    FlyerID   int64   `json:"flyer_id"`
    Name      string  `json:"name"`
    Brand     string  `json:"brand,omitempty"`
    Price     string  `json:"price,omitempty"`
    ImageURL  string  `json:"image_url,omitempty"`
    VideoURL  *string `json:"video_url,omitempty"`
}

// Store is one physical store carrying a flyer.
type Store struct {
    ID         int    `json:"id"`
    Address    string `json:"address"`
    City       string `json:"city"`
    Province   string `json:"province"`
    PostalCode string `json:"postal_code"`
}
```

Notes:

- Dates move from `string` to `time.Time`. The reconciler already parses them with `internal.ParseDate` (RFC3339). Storing as `time.Time` forces every backend to use a real date representation, not a free-form string that the JSON backend seeded as `[]string{}`.
- `FlyerItem.DisplayType` and the spatial-coordinate fields (`Left`, `Right`, `Top`, `Bottom`) are dropped from the canonical type — they are never read by any caller today (grepp'd). The flipp client still has them in its internal DTO; we just don't persist them.
- `MerchantID`, `FlyerRunID`, `FlyerTypeID`, `AvailableFrom`, `AvailableTo` are dropped from the persisted `Flyer` — the reconciler and `find-deals` only ever read `ID`, `Merchant`, `Name`, `ValidFrom`, `ValidTo`. (Verification: grep `flyer\.Merchant\|flyer\.Name\|flyer\.ValidFrom\|flyer\.ValidTo` in `cmd/`.)

### Sentinel errors

```go
var (
    ErrNotFound        = errors.New("storage: not found")
    ErrDuplicate       = errors.New("storage: duplicate")
    ErrInvalidArgument = errors.New("storage: invalid argument")
)
```

- `ErrNotFound` replaces both the existing `RetailGroupNotFound` and the `fmt.Errorf("flyer with ID %d not found", ...)` style.
- `ErrDuplicate` replaces the manual dedup loops (backends can use a real unique constraint instead of load-mutate-save).
- Backends **must** wrap these with `errors.Is` testability in mind.

### Factory `Open`

```go
func Open(ctx context.Context, opts Options) (Storage, error) {
    switch opts.Backend {
    case BackendJSON:
        return openJSON(ctx, opts)
    case BackendSQLite:
        return openSQLite(ctx, opts)
    case BackendBolt:
        return openBolt(ctx, opts)
    default:
        return nil, fmt.Errorf("%w: unknown backend %q", ErrInvalidArgument, opts.Backend)
    }
}
```

The default backend (when callers don't pick) is `BackendJSON`, to preserve current behaviour.

### Reconciler contract becomes the same `Storage` interface

`internal/reconciler/scrape/types.go` currently declares its own `StorageClient` interface. After Plan 1 that interface is replaced with an alias / type-constraint pointing at `storage.Storage`, narrowed to the methods the reconciler actually uses:

```go
type StorageClient = storage.Storage // reconciler uses a subset
```

This is a strict superset of what the reconciler needs, so it works as a drop-in.

## What gets added vs changed in this plan

### Added

- `internal/storage/storage.go` — interface, `Options`, `Backend`, sentinel errors, `Open` (with backends stubbed out — Plan 2 fills in `BackendJSON`).
- `internal/storage/migrate.go` — empty interface doc / helper signature for `Migrate(ctx)`; the body is per-backend.

### Changed

- `internal/storage/types.go` — collapsed to one canonical type set; `RetailGroupNotFound` removed.
- `internal/reconciler/scrape/types.go` — `StorageClient` becomes an alias of `storage.Storage`; `RetailGroup` and `RetailGroupItem` types deleted (replaced by `storage.Flyer` / `storage.FlyerItem`).

### **Not** changed in this plan

- `internal/storage/db.go` and `jsonfilestorage.go` stay in place. Plan 2 removes them.
- Callers in `cmd/` stay on their current path. Plan 2 wires them through `storage.Open(...)`.

## Acceptance criteria

1. `go build ./...` passes with the new interface and stubbed backends.
2. `internal/storage.Storage` is the only type any non-`internal/storage` package references for persistence.
3. The canonical types in `types.go` are the only grocery/flyer/flyer-item/store types in the repo (verified by `grep -rn "type Flyer struct\|type GroceryItem struct\|type RetailGroup struct\|type RetailGroupItem struct"` — exactly one match each).
4. Every `ErrNotFound`-style error in the storage code path returns one of the three sentinels.
5. `internal/storage.Open(ctx, Options{Backend: BackendJSON, CacheDir: ...})` returns a usable `Storage` (implementation stubs return an explicit "not yet implemented" error so a misuse fails loudly, not silently).

## Open questions for the owner before execution

1. **Date type.** Should `Flyer.ValidFrom`/`ValidTo` be `time.Time` (recommended) or stay as `string` to match the existing on-disk format and avoid migration work? `time.Time` is correct, but it forces every backend to make a decision (JSON RFC3339 strings, SQLite ISO-8601 text or integer epoch, BoltDB binary). Recommend `time.Time`.
2. **Embedding storage.** `GroceryItem.Embedding []float32` is a ~4 KB blob per item. BoltDB and SQLite both handle it fine. Keep as `[]float32` on the type, or move to a side-table for SQLite/Bolt and out-of-band storage? Recommend: keep on the type for the first cut; revisit only if a perf problem shows up.
3. **Config surface.** Should the backend be selectable via `STFG_STORAGE_BACKEND=json|sqlite|bolt` env var (via Viper) wired in `cmd/root.go`? Plan 2 will hardcode `BackendJSON` until Plan 3 lands; the env var can be added with Plan 3.
4. **Concurrency model.** `Storage` is shared across Cobra command invocations within a single process. Today JSON reads/writes are not safe under concurrent writes. Should Plan 1 declare an interface contract that requires backend implementations to be safe for concurrent use, or document that callers serialise? Recommend: require concurrent-safe; SQLite and BoltDB are; the JSON backend uses a per-key mutex or `flock`.
5. **`Context` everywhere.** Today JSON helpers take no `context.Context`. Adding it to the interface is a breaking change for any future caller that wants cancellation. Accept the change?