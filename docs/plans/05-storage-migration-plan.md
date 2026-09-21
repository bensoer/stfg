# Plan 5 — Clean Up Storage Abstraction in cmd/root.go

**Order:** 5 of 5 (Finalize storage abstraction)
**Status:** Draft — DONE — IMPLEMENTED
**Depends on:** Plans 1–3 (Storage interface, canonical types, JSON/SQLite/Bolt backends)
**Blocks:** Future backend selection mechanisms

## Goal

Ensure all storage access in the CLI commands is properly abstracted via the `storage.Storage` interface. The current implementation correctly uses `*json.FileStorage` as the concrete type at `cmd/root.go`'s initialization point, but we need to verify this is wired correctly and clean up any remaining direct dependencies.

## Current State Analysis

### What Works

The `storage.Storage` interface exists (`internal/storage/storage.go`) with all implementations (JSON, SQLite, Bolt) satisfying it. Commands use the interface correctly via `getStore()`.

### What Needs Attention

1. **`cmd/root.go`** imports `stfg/internal/storage/json` directly and calls `json.NewJSON()`
2. **Fallback in `getStore()`** also calls `json.NewJSON()` directly  
3. Need to verify there are no mixed read/write patterns across different storage implementations

## Investigation: Mixed Read/Write Patterns

After reviewing the codebase:

| Component | Storage Usage Pattern | Status |
|---|---|---|
| `cmd/addGrocery.go` | `store.AddGrocery(ctx, item)` via interface | ✅ Correct |
| `cmd/listGroceries.go` | `store.ListGroceries(ctx)` via interface | ✅ Correct |
| `cmd/removeGrocery.go` | `store.RemoveGrocery(ctx, item)` via interface | ✅ Correct |
| `cmd/scrapeFlyers.go` | `store.AddFlyer()`, `store.AddFlyerItem()` via interface | ✅ Correct |
| `cmd/findDeals.go` | `store.ListFlyers()`, `store.ListGroceries()`, `store.ListFlyerItems()` via interface | ✅ Correct |
| `internal/reconciler/scrape/engine.go` | All via `storage.Storage` interface | ✅ Correct |
| `internal/promptwriter/openrouterfree.go` | Uses `storage.FlyerItem`, `storage.GroceryItem` types | ✅ Correct types |

**No mixed read/write patterns found.** All operations go through the `storage.Storage` interface consistently.

## The Actual Issue

The only "problem" is that `cmd/root.go` has a direct import of `stfg/internal/storage/json` and calls `json.NewJSON()` directly. This is:
- Functionally correct for the JSON backend
- Properly abstracted through the interface for downstream calls
- Simply the concrete initialization point

## Proposed Changes

Since the user specified "DO NOT use a factory pattern" and wants to keep the JSON backend wired directly, here's what we should do:

### Option A: Keep Current Approach (Simplest)

The current code is already correct for the JSON backend:

```go
// cmd/root.go - CURRENT (minimal change needed)
import (
    "stfg/internal/storage"
    "stfg/internal/storage/json"
)

func init() {
    // ...
    rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
        // ...
        store, err := json.NewJSON(cmd.Context())  // Direct JSON init, OK
        // ...
    }
}
```

This is fine - it's just the concrete initialization, all downstream uses are interface-based.

### Option B: Refactor getStore() Fallback (Recommended)

The `getStore()` fallback is rarely used but could be cleaner:

```go
func getStore(cmd *cobra.Command) storage.Storage {
    store, ok := cmd.Context().Value(storageCtxKey{}).(storage.Storage)
    if ok && store != nil {
        return store
    }
    // Fallback: direct JSON init (same pattern as PersistentPreRunE)
    store, err := json.NewJSON(cmd.Context())
    if err != nil {
        return nil
    }
    ctx := context.WithValue(cmd.Context(), storageCtxKey{}, store)
    cmd.SetContext(ctx)
    return store
}
```

This is already the current implementation - no change needed.

## Recommendation

**No changes needed for functional correctness.** The storage abstraction is working:

1. `cmd/root.go` initializes storage via `json.NewJSON()` - returns `*json.FileStorage`
2. `*json.FileStorage` implements `storage.Storage` interface
3. All commands use `getStore()` which returns `storage.Storage` interface
4. All storage operations are called on the interface, not concrete type

The only consideration is whether to remove the `stfg/internal/storage/json` import. This would require either:
- A factory pattern (user rejected)
- Making `json.NewJSON` part of the `storage` package (changes interface)
- Keeping the import (current state)

## Acceptance Criteria

1. ✅ All command files use `storage.Storage` interface (verified)
2. ✅ No mixed read/write patterns across backends (verified)  
3. ✅ All storage methods properly wrap errors as sentinel errors (verified via parity tests)
4. ✅ JSON backend properly implements all interface methods (verified)
5. Pending: Decide on `json` package import in `cmd/root.go`

## Files Reviewed

- `cmd/root.go` - storage initialization (2 direct `json.NewJSON()` calls)
- `cmd/addGrocery.go` - uses `store.AddGrocery(ctx, storage.GroceryItem{...})` interface
- `cmd/listGroceries.go` - uses `store.ListGroceries(ctx)` interface
- `cmd/removeGrocery.go` - uses `store.RemoveGrocery(ctx, item)` interface
- `cmd/scrapeFlyers.go` - uses `store.AddFlyer()`, `store.HasFlyer()`, `store.HasFlyerItem()` interface
- `cmd/findDeals.go` - uses `store.ListFlyers()`, `store.ListGroceries()`, `store.ListFlyerItems()` interface
- `internal/reconciler/scrape/engine.go` - uses `storage.Storage` interface
- `internal/promptwriter/openrouterfree.go` - uses `storage.FlyerItem`, `storage.GroceryItem` types
- `internal/storage/json/jsonfilestorage.go` - implements `storage.Storage` interface
- `internal/storage/storage.go` - defines `Storage` interface

## Conclusion

The storage abstraction is **already properly implemented**. The `cmd/root.go` file's direct import of `json.NewJSON()` is simply the concrete initialization at the application's entry point - all downstream calls use the `storage.Storage` interface.

If the user wants to remove the `json` import, they need to either:
1. Accept a factory pattern (rejected)
2. Accept a registry pattern (need clarification)
3. Move `NewJSON` into the storage package and have `Options` select the backend

Without a factory/registry pattern, we cannot remove the `json` import while maintaining the current architecture.