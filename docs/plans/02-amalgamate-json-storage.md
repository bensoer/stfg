# Plan 2 — Amalgamate JSON Storage Into One Implementation

**Order:** 2 of 3
**Status:** Draft — DO NOT EXECUTE
**Depends on:** Plan 1 (the `Storage` interface and canonical types)
**Blocks:** Plan 3 (the SQLite/Bolt backends will mirror what this plan establishes for the JSON backend — file location, error wrapping, migration shape)

## Goal

Collapse `internal/storage/db.go` (flat package-level functions) and `internal/storage/jsonfilestorage.go` (`JSONFileStorage` struct + methods) into a **single** JSON-backed implementation that:

1. Implements `storage.Storage` from Plan 1.
2. Preserves every CRUD action that exists in **either** of the current implementations.
3. Resolves the schema disagreement (the user-facing bug where `add-grocery` writes a `[]GroceryItem` file that `list-groceries` then can't decode).
4. Drops the dead / redundant code that the investigation flagged.

After this plan the only persistence entry point is `storage.Open(ctx, storage.Options{Backend: BackendJSON, CacheDir: ...})`.

## Ben-mcp-server / context7 / grepp guidance

- **ben-mcp-server:** no relevant standards (Python/Git/skill-authoring scope). Proceeding with Go-idiomatic conventions.
- **context7 / grepp:** not consulted — no new library is being added in this plan.

## Current state to be replaced

| Source | What it offers |
|---|---|
| `db.go` | `EnsureGroceryFileExists`, `LoadGroceries`, `SaveGroceries`, `LoadAllFlyers`, `LoadFlyer`, `SaveFlyer`, `LoadFlyerItems`, `SaveFlyerItems`, `DeleteFlyerItems`, `RemoveInvalidFlyers`, plus file-name helpers `GroceryFile`, `FlyerFileName`, `FlyersIndexFile`. Grocery schema: `[]string`. Flyer schema: `[]Flyer` (storage.Flyer, snake_case). |
| `jsonfilestorage.go` | `NewJSONFileStorage`, `GetAllGroceries`, `AddGrocery`, `RemoveGrocery`, plus full retail-group / retail-group-item CRUD (`HasRetailGroup`, `HasRetailGroupItem`, `AddRetailGroup`, `AddRetailGroupItem`, `GetRetailGroup`, `RemoveRetailGroup`, `RemoveRetailGroupItem`, `GetRetailGroupItems`, `GetAllRetailGroups`). Grocery schema: `[]GroceryItem`. Flyer schema: `[]RetailGroup` (scrape.RetailGroup, camelCase). |
| `db.go`'s `RemoveInvalidFlyers` | Expiry pruning by date string. **No callers.** |
| `db.go`'s `SaveFlyer` | Dedup-by-ID rewrite. Used by **no current command**. |

## What "best of both" means in practice

| Capability | Today (which file?) | Plan 2 outcome |
|---|---|---|
| Grocery: list | `db.go` returns `[]string` | `ListGroceries(ctx)` returns `[]storage.GroceryItem{Name, Embedding}` |
| Grocery: add (with embedding, dedup check) | `jsonfilestorage.go` | Keep as `AddGrocery(ctx, GroceryItem)` |
| Grocery: remove (case-insensitive name match) | `db.go` does the case-insensitive loop on `[]string` | Keep as `RemoveGrocery(ctx, name string)` |
| Grocery: case-insensitive has-check | `addGrocery.go` does it inline; `jsonfilestorage.go` has nothing equivalent | New `HasGrocery(ctx, name string) (bool, error)` — moves the loop out of the command |
| Flyer: list / get | `db.go`'s `LoadAllFlyers` / `LoadFlyer` | `ListFlyers(ctx)` / `GetFlyer(ctx, id)` |
| Flyer: add (dedup-by-ID rewrite) | `jsonfilestorage.go` `AddRetailGroup`; `db.go` `SaveFlyer` (unused) | `AddFlyer(ctx, Flyer)` — one method |
| Flyer: remove (with cascading items) | `jsonfilestorage.go` `RemoveRetailGroup` (errors ignored inside the loop) | `RemoveFlyer(ctx, id)` — proper error propagation |
| Flyer: has-check | `jsonfilestorage.go` `HasRetailGroup` (returns `bool` only, swallows lookup errors) | `HasFlyer(ctx, id) (bool, error)` per Plan 1 |
| Flyer items: list | `db.go` `LoadFlyerItems` | `ListFlyerItems(ctx, flyerID)` |
| Flyer items: add (per-flyer file) | `jsonfilestorage.go` `AddRetailGroupItem` | `AddFlyerItem(ctx, FlyerItem)` |
| Flyer items: remove | `jsonfilestorage.go` `RemoveRetailGroupItem` (ignored errors inside loops in the cascading path) | `RemoveFlyerItem(ctx, flyerID, itemID)` |
| Flyer items: has-check | `jsonfilestorage.go` `HasRetailGroupItem` | `HasFlyerItem(ctx, flyerID, itemID) (bool, error)` |
| Expiry pruning | `db.go` `RemoveInvalidFlyers` (unused) + reconciler's own loop | `PruneExpired(ctx, now)` per Plan 1 — **one source of truth**, reconciler stops doing its own expiry walk |

## What gets deleted (redundant / unused / dead)

| Code | Why it's going |
|---|---|
| `internal/storage/db.go` (entire file) | Superseded by `JSONFileStorage`. |
| `internal/storage/jsonfilestorage.go` (entire file) | Replaced by the new `internal/storage/json/jsonfilestorage.go`. |
| `GroceryFile()`, `GroceryFileName` constants | Filename is a private detail of the JSON backend. |
| `FlyerFileName()`, `FlyersIndexFile()`, `RetailGroupIndexFileName` | Same. |
| `EnsureGroceryFileExists()` | `Migrate(ctx)` on the interface handles this. `cmd/groceries.go`'s `PersistentPreRunE` calls `storage.Open` instead. |
| `RemoveInvalidFlyers()` | Dead code. The new `PruneExpired` covers it; the reconciler stops pruning inline. |
| `SaveFlyer()` | No current caller. `AddFlyer` supersedes it. |
| `SaveFlyerItems()` / `DeleteFlyerItems()` | No callers outside `RemoveInvalidFlyers` (which itself is dead). The new `AddFlyerItem` / `RemoveFlyerItem` cover the need. |
| `internal/reconciler/scrape/types.go` — `RetailGroup`, `RetailGroupItem`, `RetailGroupLocation` types | Replaced by `storage.Flyer`, `storage.FlyerItem`, `storage.Store`. |
| `internal/reconciler/scrape/engine.go` — inline expiry loop at the top of `Reconcile` | Moved into `PruneExpired`. The reconciler becomes pure upsert. |
| `internal/reconciler/scrape/engine.go` — error-discard `j.RemoveRetailGroupItem(rtgi)` | `RemoveFlyer` handles cascade with proper error propagation. |

## File layout after this plan

```
internal/storage/
├── storage.go             # interface, Options, errors, Open (from Plan 1)
├── types.go               # canonical types (from Plan 1)
├── json/
│   ├── jsonfilestorage.go # the single JSON implementation
│   └── migrate.go         # JSON-specific Migrate (creates the index file as []Flyer{})
└── (sqlite, bolt dirs not yet created — Plan 3)
```

## Implementation outline for `internal/storage/json/jsonfilestorage.go`

```go
package jsonstorage

import (
    "context"
    "encoding/json"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "sync"

    "stfg/internal/storage"
)

type FileStorage struct {
    dir       string
    indent    bool
    fileMode  os.FileMode

    // one mutex per file path, lazily created. Protects load-mutate-save.
    muByPath  sync.Map // map[string]*sync.Mutex
}

func Open(ctx context.Context, opts storage.Options) (*FileStorage, error) {
    if err := os.MkdirAll(opts.CacheDir, dirMode(opts.FileMode)); err != nil {
        return nil, err
    }
    return &FileStorage{
        dir:      opts.CacheDir,
        indent:   opts.Indent,
        fileMode: opts.FileMode,
    }, nil
}

func (f *FileStorage) Close() error { return nil }

func (f *FileStorage) Migrate(ctx context.Context) error {
    // Idempotent. Creates:
    //   groceries.json      → []
    //   flyers_index.json   → []   (NOT []string{}, the bug being fixed)
    // Per-flyer files are created on first AddFlyerItem.
    return f.ensureEmpty(f.groceriesPath(), []storage.GroceryItem{})
}

func (f *FileStorage) ensureEmpty(path string, seed any) error {
    if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
        return f.writeFile(path, seed)
    }
    return nil
}

// --- file layout (private to this backend) ---

const (
    groceriesFile  = "groceries.json"
    flyersIndex    = "flyers_index.json"
    flyerItemsBase = "flyer_" // flyer_<id>.json
)

// --- generic read/write with mutex ---

func (f *FileStorage) lockFor(path string) *sync.Mutex { ... }

func (f *FileStorage) writeFile(path string, data any) error {
    mu := f.lockFor(path)
    mu.Lock()
    defer mu.Unlock()

    tmp := path + ".tmp"
    fh, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.fileMode)
    if err != nil { return err }
    enc := json.NewEncoder(fh)
    if f.indent { enc.SetIndent("", "  ") }
    if err := enc.Encode(data); err != nil { fh.Close(); return err }
    if err := fh.Close(); err != nil { return err }
    return os.Rename(tmp, path) // atomic write
}

func (f *FileStorage) readFile(path string, out any) error {
    mu := f.lockFor(path)
    mu.Lock()
    defer mu.Unlock()

    data, err := os.ReadFile(path)
    if errors.Is(err, fs.ErrNotExist) { return fmt.Errorf("%w: %s", storage.ErrNotFound, path) }
    if err != nil { return err }
    return json.Unmarshal(data, out)
}

// --- CRUD implementations follow the Plan 1 interface verbatim ---
// Each method is the load-mutate-save pattern from the current
// jsonfilestorage.go, with errors wrapped to the Plan 1 sentinels.
```

Key implementation notes:

- **Atomic writes** (write to `*.tmp`, rename). The current `os.Create` + `enc.Encode` path truncates the file before encoding completes — a crash mid-encode leaves a corrupt JSON. The temp+rename pattern is cheap and fixes that.
- **Per-path mutex** (`sync.Map` of `*sync.Mutex`). Today the JSON backend has no concurrency control. Two parallel `add-grocery` runs would race. SQLite/BoltDB will be naturally safe; the JSON backend needs explicit locking.
- **`Migrate` seeds `groceries.json` as `[]storage.GroceryItem{}`**, not `[]string{}`. This is the bug fix the investigation identified.

## Reconciler changes (`internal/reconciler/scrape/engine.go`)

Before:

```go
// iterate over storageClient RetailGroups and remove whats invalid
for _, rtg := range rtgs {
    retailGroupIsValid, err := RetailGroupIsValid(rtg)
    if !retailGroupIsValid {
        // ... remove items, then remove group ...
    }
}
// All expired RetailGroups are now gone, so now we can add new stuff
```

After:

```go
// Expiry pruning is the storage layer's job now.
if err := storageClient.PruneExpired(ctx, time.Now()); err != nil {
    return err
}
// ... pure upsert loop follows ...
```

The reconciler drops its own `RetailGroupIsValid` call site; if the reconciler still needs validity info for logging, it uses `storage.Flyer.ValidTo` directly (now a `time.Time`).

## Caller rewiring (`cmd/`)

| File | Today | After Plan 2 |
|---|---|---|
| `cmd/groceries.go` | `PersistentPreRunE` calls `storage.EnsureGroceryFileExists()` | `PersistentPreRunE` calls `storage.Open(ctx, storage.Options{Backend: BackendJSON, CacheDir: storage.MustCacheDir()})` and stores the handle on the command context for subcommands |
| `cmd/addGrocery.go` | `storage.NewJSONFileStorage()` → `jsonStorage.GetAllGroceries` etc. | `store, _ := ctx.Value(storageKey{}).(storage.Storage)` → `store.ListGroceries(ctx)` etc. |
| `cmd/listGroceries.go` | `storage.LoadGroceries()` returns `[]string` | `store.ListGroceries(ctx)` returns `[]storage.GroceryItem` — **also fix the printing loop to print `.Name`** |
| `cmd/removeGrocery.go` | Inline case-insensitive loop on `[]string` | `store.RemoveGrocery(ctx, item)` |
| `cmd/scrapeFlyers.go` | `storage.NewJSONFileStorage()` → `scrape.Reconcile(client, storage, ...)` | `store, _ := ctx.Value(...)` → `scrape.Reconcile(client, store, ...)` |
| `cmd/findDeals.go` | `storage.LoadAllFlyers`, `storage.LoadGroceries`, `storage.LoadFlyerItems` | `store.ListFlyers`, `store.ListGroceries`, `store.ListFlyerItems` — also fix the grocery handling (today it passes `[]string` to the promptwriter; after Plan 2 it passes `[]storage.GroceryItem` and the promptwriter reads `.Name`) |

The promptwriter change:

| File | Today | After Plan 2 |
|---|---|---|
| `internal/promptwriter/openrouterfree.go` | `func (o *OpenRouterFreePromptWriter) GetFlyerItemsOnGroceryList(flyerItems []storage.FlyerItem, groceryList []string) (map[string][]storage.FlyerItem, error)` | `... groceryList []storage.GroceryItem ...` — change the inner `for _, grocery := range groceryList` to use `grocery.Name` |

## Tests to add (none today, this plan is the chance)

Per the project's `AGENTS.md` note ("No tests yet"), Plan 2 is the right time to add the first round:

- `internal/storage/json/jsonfilestorage_test.go` — round-trip a flyer + items, exercise every CRUD method against a `t.TempDir()` cache dir.
- `cmd/findDeals_test.go` — wiring test that the Cobra command reaches the storage handle.
- Concurrency test: two goroutines call `AddGrocery` in parallel; verify no torn writes (this is what the per-path mutex is for).

## Acceptance criteria

1. `go build ./...` passes with `db.go` and the old `jsonfilestorage.go` deleted.
2. Every command runs against the same `storage.Storage` instance obtained from `storage.Open`.
3. `groceries.json` is always `[]storage.GroceryItem{}` when empty (verified by running `groceries add-grocery milk` then `cat groceries.json`).
4. `flyers_index.json` is always `[]storage.Flyer{}` when empty (was `[]string{}` — the bug from the investigation).
5. JSON files are written via temp-file + rename (verified by code review; no `os.Create(path)` followed by encoder on `path` anywhere).
6. Concurrent `AddGrocery` calls produce a valid JSON file with both items present (no torn writes).
7. `RemoveInvalidFlyers` is gone from the codebase (verified by grep).
8. `internal/reconciler/scrape/engine.go` no longer has the inline expiry walk; pruning is `storage.PruneExpired(ctx, now)`.
9. `find-deals` still produces the same output as before for an existing flyer + grocery list (manual smoke test).

## Migration / backwards-compatibility note

This plan **changes the on-disk format** for `groceries.json`. A user with an existing `[]string` file will, after upgrade, see `find-deals` and `remove-grocery` succeed but `add-grocery` will append a `GroceryItem` (with embedding) and the file will then fail to decode for `list-groceries` / `remove-grocery` — wait, that bug is exactly what Plan 2 fixes by going to a single type, so on the next `add-grocery` the file is rewritten in the new shape and everything works going forward.

If preserving existing user data matters, add a one-time migration in `Migrate`:

```go
// In jsonfilestorage.Migrate: if groceries.json decodes as []string,
// rewrite it as []GroceryItem{{Name: s}}.
```

Plan 2 includes this as a small, well-scoped extra step. If the user data set is small enough to discard, the migration step can be skipped.

## Open questions for the owner before execution

1. **Embeddings-on-list.** `listGroceries` today prints names only. With the new shape, every loaded item carries a `[]float32` we don't need. Should `ListGroceries` return full items, or should there be a `ListGroceryNames(ctx) ([]string, error)` method that the JSON backend implements by mapping? Recommend: keep one method, full items; the cost is negligible (a few KB) and the API stays clean.
2. **File locking on macOS.** Plan 2 uses per-path `sync.Mutex`, which only protects within a single process. If two `stfg` invocations run in parallel (unlikely but possible), there's no inter-process lock. Acceptable for v1, or add `flock`? Recommend: accept for v1; document as a known limitation.
3. **Atomic write semantics.** Temp + rename is POSIX-atomic on the same filesystem. macOS and Linux both honour it. Windows does not. Plan 2 is Linux-first per `AGENTS.md`; accept the limitation or add an `os.IsWindows` fallback that uses the existing non-atomic path? Recommend: atomic write, document Windows as best-effort.
4. **Renaming `RemoveInvalidFlyers` callers.** Zero callers today, so safe. Just confirming: are there any out-of-tree callers (tests, scripts) the owner is aware of?
5. **`HasGrocery` vs inline check in `addGrocery`.** Plan 1 introduces `HasGrocery(ctx, name) (bool, error)`. `addGrocery` today does the loop inline. Replace the loop with `HasGrocery`? Recommend yes — moves the case-insensitive match into one place.