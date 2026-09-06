# Plan 4 — FlyerFinder Abstraction

**Order:** 4 of N (this worktree's plans start at 04 to avoid filename
collision with the persistence-abstraction worktree's plans 01–03, which
cover unrelated work)
**Status:** Draft — DO NOT EXECUTE
**Depends on:** nothing
**Blocks:** any future `StoreCatalog` / regular-price-comparison plan

## Goal

Introduce a `FlyerFinder` interface as the single contract for
"source of flyer data near a postal code." The current implementation is the
Flipp API client (`internal/flyerfinder/flipp`); the contract is designed so that
alternative implementations (a different aggregator, a CSV importer, a test
mock) can be substituted without changing the reconciler or any command.

After this plan:

- A new package `internal/flyerfinder` declares the interface.
- `internal/flipp` is **moved** to `internal/flyerfinder/flipp` and reworked
  to satisfy the interface and to construct canonical `storage.Flyer` /
  `storage.FlyerItem` records at the API boundary, replacing the current
  `scrape.RetailGroup*` intermediate types.
- The reconciler (`internal/reconciler/scrape`) depends on
  `flyerfinder.FlyerFinder` and `storage.Storage` only — not on `flipp`,
  not on `RetailGroup*` types.
- `cmd/scrapeFlyers` depends on `flyerfinder.FlyerFinder` only — not on
  `flipp.Client` directly.
- The hardcoded retailer whitelist moves into viper under
  `fly_finder.whitelist`.

## Why this comes first

Today the reconciler already declares a `ScrapeClient` interface, but:

1. It lives in `internal/reconciler/scrape/types.go` — tucked under the
   reconciler rather than at the project root of the abstraction.
2. Its methods return `scrape.RetailGroup` / `scrape.RetailGroupItem`,
   which are intermediate types owned by the reconciler package. Any
   implementation of `ScrapeClient` is forced to import the reconciler
   package, inverting the dependency direction.
3. The reconciler also declares its own `StorageClient` interface that
   `JSONFileStorage` happens to satisfy, but is not the same as
   `storage.Storage` from the persistence-abstraction plans. This work
   does not yet land persistence Plan 1 (out of scope below), so for now
   the reconciler keeps using `JSONFileStorage` directly — but typed as
   the *same* signature shape, ready to swap to `storage.Storage` later.

## Architecture

```
   ┌────────────────────────┐
   │  FlyerFinder           │   ← this plan
   │  (internal/flyerfinder)│
   └────────────┬───────────┘
                │  FindFlyers / FindFlyerItems
                │  returns []storage.Flyer, []storage.FlyerItem
                ▼
   ┌────────────────────────┐
   │  flipp.Client          │   ← current implementation
   │  (internal/flyerfinder/│      maps Flipp DTOs -> canonical types
   │   flipp)               │      sub-package layout mirrors
   └────────────────────────┘      internal/storage/{json,sqlite,bolt}

   The reconciler and `cmd/scrapeFlyers` depend only on FlyerFinder and
   (post-persistence-Plan-1) storage.Storage. Future StoreCatalog
   abstraction is intentionally separate — see "Future Work" below.
```

## Ben-mcp-server / context7 / grepp guidance

- **ben-mcp-server:** no relevant standards for this work. The published
  standards cover Python, Git workflows, and skill authoring only. Go
  architecture decisions proceed from the standard Go-idiomatic conventions
  (small interfaces in the consumer's package, or in a dedicated
  interface-only package; "accept interfaces, return structs"; compile-time
  interface assertions with `var _ Iface = (*T)(nil)`).
- **context7 / grepp:** not consulted. No new library is being added in
  this plan; the changes are mechanical refactors of existing types and a
  new interface declaration.

## Compatibility with persistence-abstraction work

This worktree has not yet landed persistence Plans 1–3. Three
forward-compatibility decisions preserve the persistence plans' shape:

1. **`FlyerFinder` returns `[]storage.Flyer` and `[]storage.FlyerItem`,**
   not the current `scrape.RetailGroup*`. This matches the canonical
   types the persistence-abstraction worktree already defined and shipped
   in its `internal/storage/types.go`.

2. **The reconciler's `Reconcile` signature takes a `storage.Storage`-shaped
   parameter.** For now this worktree still has the unconsolidated storage
   (`db.go` + `jsonfilestorage.go`), so the parameter is typed as the
   concrete `*storage.JSONFileStorage`. When persistence Plan 2 lands, this
   becomes `storage.Storage` mechanically — a type rename in one
   signature and one assignment in `cmd/scrapeFlyers`. The reconciler
   code body needs no further change.

3. **`storage.Flyer.ValidFrom` / `ValidTo` stay as `string` in this
   worktree.** The persistence-abstraction worktree flips them to
   `time.Time`. We do not flip them here because the on-disk format and
   `db.go` writers both encode them as strings; flipping the type without
   flipping the persistence path would create a runtime parse failure. The
   flipp client parses `*string` from Flipp's JSON and writes the parsed
   string into `storage.Flyer.ValidFrom`. When the persistence plans
   land, both flip together.

Out of scope but acknowledged: the persistence-abstraction worktree's
Plan 1 already added the Storage interface, sentinel errors, and the
`PruneExpired` method. This plan does NOT add `PruneExpired`. The
reconciler's existing inline expiry walk stays in place. See "Expiry
handling" below.

## Scope

### In scope

- New package `internal/flyerfinder` with the `FlyerFinder` interface.
- **Move** `internal/flipp` → `internal/flyerfinder/flipp`. The Flipp
  implementation becomes a sub-package of `flyerfinder`, mirroring
  `internal/storage/json/`, `internal/storage/sqlite/`, and
  `internal/storage/bolt/`. The dependency direction is
  `flyerfinder/flipp` → `flyerfinder` (sub-package imports parent to
  know the interface). No risk of circular import.
- Rework `internal/flyerfinder/flipp`:
  - Rename DTO `StoreLocation` → `RetailGroupLocation` for naming
    consistency with `Flyer` / `FlyerItem` DTOs already in the package.
  - Add `NewFinder()` returning `*Client` (struct name unchanged).
  - Methods `FindFlyers(postalCode) ([]storage.Flyer, error)` and
    `FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error)`.
  - Compile-time assertion `var _ flyerfinder.FlyerFinder = (*Client)(nil)`.
- Rework `internal/reconciler/scrape`:
  - Delete `ScrapeClient` interface.
  - Delete `StorageClient` interface.
  - Delete `RetailGroup`, `RetailGroupItem`, `RetailGroupLocation` types.
  - Rename method `GetRetailGroups` → `FindFlyers`, `GetRetailGroupItems` →
    `FindFlyerItems` in the flipp client (interface method names).
  - Variable renames in `engine.go`: `scrapeClient` → `finder`,
    `retailGroup` → `flyer`, `retailGroupItem` → `item`.
  - `Reconcile` signature: `(ctx context.Context, finder flyerfinder.FlyerFinder, store *storage.JSONFileStorage, options ScrapeReconcilerOptions) error`.
- Rework `cmd/scrapeFlyers`:
  - `finder := flipp.NewFinder()` instead of `client := flipp.NewClient()`.
  - `var f flyerfinder.FlyerFinder = finder` at the call site so the cmd
    is explicit about depending on the interface.
  - Whitelist loaded from viper key `fly_finder.whitelist` (see below).
- Rework `cmd/root.go`:
  - Seed a default `fly_finder.whitelist` value in viper if the key is
    absent from config. The default is the current hardcoded list
    (Superstore, Thrify Foods, Quality Foods, Buy-Low Foods, Country Grocer,
    No Frills, Pharmasave, Shoppers Drug Mart, Walmart, Nesters Market,
    Rexall).

### Out of scope

- The `db.go` vs `jsonfilestorage.go` schema split, sentinel errors,
  `PruneExpired`, and the canonical `Storage` interface — those are
  persistence Plan 2. This plan does not touch `internal/storage/db.go`
  or `internal/storage/jsonfilestorage.go` beyond the parameter-type change
  in the reconciler signature.
- `storage.Flyer.ValidFrom` / `ValidTo` flipping `string` → `time.Time` —
  persistence Plan 1's concern.
- The `findDeals` command — does not touch flipp; already abstracted at
  the storage layer.
- `cmd/groceries` (add/list/remove grocery) — does not touch flipp.
- A `StoreCatalog` interface for regular-price observation at stores,
  and any "comparator" layer that consumes FlyerFinder + StoreCatalog —
  future plans. See "Future Work".

## Proposed design

### File layout after this plan

```
internal/
├── flyerfinder/                      # NEW
│   ├── flyerfinder.go                # the FlyerFinder interface
│   ├── flyerfinder_test.go           # mock + compile-time assertion
│   └── flipp/                        # MOVED from internal/flipp (now a sub-package)
│       ├── client.go                 # FindFlyers / FindFlyerItems; returns storage.Flyer/FlyerItem
│       ├── types.go                  # Flyer, FlyerItem DTOs (unchanged shape);
│       │                             # StoreLocation renamed -> RetailGroupLocation
│       ├── client_test.go            # NEW: httptest-based coverage
│       └── utils.go                  # unchanged
├── reconciler/
│   └── scrape/
│       ├── engine.go                 # Reconcile takes flyerfinder.FlyerFinder + *storage.JSONFileStorage
│       ├── types.go                  # RetailGroup{,Item,Location}, ScrapeClient, StorageClient all deleted
│       └── engine_test.go            # NEW: signature + interface-typed-parameter tests
└── storage/                          # UNCHANGED: db.go + jsonfilestorage.go split stays until persistence Plan 2

cmd/
├── scrapeFlyers.go                   # imports flyerfinder + flyerfinder/flipp;
                                      # whitelist from viper; FlyerFinder at call site
└── root.go                           # seed default fly_finder.whitelist
```

The layout mirrors the persistence-abstraction worktree's
`internal/storage/` design: the interface lives at the package root
(`internal/flyerfinder/`); each implementation lives in a sub-package
(`internal/flyerfinder/flipp/` for now; future `internal/flyerfinder/storecatalog/`,
`internal/flyerfinder/csvimport/`, etc. without further changes to this
layout).

### The `FlyerFinder` interface

```go
// internal/flyerfinder/flyerfinder.go
package flyerfinder

import "stfg/internal/storage"

// FlyerFinder is the abstract source of flyer data near a postal code.
//
// The current implementation is internal/flyerfinder/flipp (the Flipp
// enterprise API).
// Any backend that can return canonical storage.Flyer and storage.FlyerItem
// records for a postal code can satisfy this interface — including a
// different aggregator, a CSV importer, or a mock for tests.
type FlyerFinder interface {
    // FindFlyers returns all flyers available near the given postal code.
    // The caller is expected to filter for relevance and validity.
    FindFlyers(postalCode string) ([]storage.Flyer, error)

    // FindFlyerItems returns all items inside one flyer.
    FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error)
}
```

Notes:

- The interface is intentionally narrow: two methods, both with simple
  inputs and canonical-type outputs.
- No `Close()` or `Init()` methods — implementations are expected to
  construct themselves cheaply (matching the existing flipp.Client pattern
  where `NewClient` returns a configured struct). If a future
  implementation needs lifecycle hooks (connection pools, rate limiters),
  the interface can be extended; for now YAGNI.
- No `Whitelist` parameter — that's a caller concern (the reconciler's
  filter, the cmd's viper-loaded config), not the finder's.

### flipp.Client rework

```go
// internal/flyerfinder/flipp/client.go
package flipp

import (
    "encoding/json"
    "fmt"

    "github.com/h2non/gentleman"

    "stfg/internal/flyerfinder"
    "stfg/internal/storage"
)

type Client struct {
    base *gentleman.Client
}

// NewFinder returns a FlyerFinder backed by the Flipp enterprise API.
// The returned concrete type is *Client, satisfying the flyerfinder.FlyerFinder
// interface. Go idiom: producers return structs, consumers accept interfaces.
func NewFinder() *Client {
    return &Client{
        base: gentleman.New(),
    }
}

// Compile-time assertion that *Client satisfies flyerfinder.FlyerFinder.
var _ flyerfinder.FlyerFinder = (*Client)(nil)

func (c *Client) FindFlyers(postalCode string) ([]storage.Flyer, error) {
    resp, err := c.GetFlyers(postalCode)
    if err != nil {
        return nil, err
    }

    flyers := []storage.Flyer{}
    for _, flippFlyer := range resp.Flyers {

        storeResp, err := c.GetNearbyStores(flippFlyer.ID, postalCode)
        if err != nil {
            return nil, err
        }

        stores := []storage.Store{}
        for _, store := range *storeResp {
            stores = append(stores, storage.Store{
                ID:         store.ID,
                Address:    store.Address,
                City:       store.City,
                PostalCode: store.PostalCode,
                Province:   store.Province,
            })
        }

        // Resolve the effective valid-from and valid-to strings. Prefer the
        // explicit ValidFrom/ValidTo with AvailableFrom/AvailableTo as
        // fallback. Skip the flyer if both are nil to avoid a nil-pointer
        // dereference. The strings are kept as RFC3339 (storage.Flyer
        // stores ValidFrom/ValidTo as string today).
        validFrom := flippFlyer.ValidFrom
        if validFrom == nil {
            validFrom = flippFlyer.AvailableFrom
        }
        if validFrom == nil {
            continue
        }

        validTo := flippFlyer.ValidTo
        if validTo == nil {
            validTo = flippFlyer.AvailableTo
        }
        if validTo == nil {
            continue
        }

        flyers = append(flyers, storage.Flyer{
            ID:        flippFlyer.ID,
            ValidFrom: *validFrom,
            ValidTo:   *validTo,
            Name:      flippFlyer.Name,
            Merchant:  flippFlyer.Merchant,
            Stores:    stores,
        })
    }

    return flyers, nil
}

func (c *Client) FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error) {
    resp, err := c.GetFlyerItems(flyerID)
    if err != nil {
        return nil, err
    }

    items := []storage.FlyerItem{}
    for _, flippItem := range *resp {

        var videoURL string
        if flippItem.VideoURL != nil {
            videoURL = *flippItem.VideoURL
        }

        items = append(items, storage.FlyerItem{
            ID:          flippItem.ID,
            FlyerID:     flyerID,
            Name:        flippItem.Name,
            Brand:       flippItem.Brand,
            Price:       flippItem.Price,
            CutoutImageURL: flippItem.CutoutImageURL,
            VideoURL:    videoURL,
        })
    }

    return items, nil
}

// GetFlyers, GetFlyerItems, GetNearbyStores stay as-is — they're the
// raw HTTP DTO methods. The new FindFlyers / FindFlyerItems compose them.
```

### flipp/types.go — DTO rename

```go
// Was: StoreLocation. Now: RetailGroupLocation.
// Field set and JSON tags unchanged.
type RetailGroupLocation struct {
    ID         int    `json:"id"`
    Address    string `json:"address"`
    City       string `json:"city"`
    Province   string `json:"province"`
    PostalCode string `json:"postal_code"`
}

// GetStoresNearByResponse's element type is now RetailGroupLocation.
type GetStoresNearByResponse []RetailGroupLocation
```

### Reconciler rework

```go
// internal/reconciler/scrape/types.go (after)
package scrape

type ScrapeReconcilerOptions struct {
    PostalCode           string
    RetailGroupWhiteList []string
}

// ScrapeClient, StorageClient, RetailGroup, RetailGroupItem,
// RetailGroupLocation — all deleted.

// internal/reconciler/scrape/engine.go (after)
package scrape

import (
    "context"
    "fmt"
    "strings"
    "time"

    "stfg/internal"
    "stfg/internal/flyerfinder"
    "stfg/internal/storage"

    "go.uber.org/zap"
)

func Reconcile(
    ctx context.Context,
    finder flyerfinder.FlyerFinder,
    store *storage.JSONFileStorage,
    options ScrapeReconcilerOptions,
) error {
    log := zap.S()

    // iterate over stored flyers and remove what's invalid
    flyers, err := store.GetAllRetailGroups() // still camelCase-ish; method names unchanged in this worktree
    if err != nil {
        return err
    }
    for _, flyer := range flyers {
        valid, err := FlyerIsValid(flyer)
        if err != nil {
            return err
        }
        if !valid {
            log.Infof("Flyer %s - %s Is Invalid. Removing", flyer.Merchant, flyer.Name)
            items, err := store.GetRetailGroupItems(flyer.ID)
            if err != nil {
                return err
            }
            for _, item := range items {
                if err := store.RemoveRetailGroupItem(item); err != nil {
                    return err
                }
            }
            if err := store.RemoveRetailGroup(flyer); err != nil {
                return err
            }
        }
    }

    // Fetch new flyers from the finder.
    newFlyers, err := finder.FindFlyers(options.PostalCode)
    if err != nil {
        return err
    }

    for _, flyer := range newFlyers {

        valid, err := FlyerIsValid(flyer)
        if err != nil {
            return err
        }
        if !valid {
            continue
        }

        found := false
        for _, whitelist := range options.RetailGroupWhiteList {
            fullName := fmt.Sprintf("%s %s", flyer.Merchant, flyer.Name)
            if strings.Contains(fullName, whitelist) {
                found = true
                break
            }
        }
        if !found {
            continue
        }

        if !store.HasRetailGroup(flyer) {
            log.Infof("Adding Flyer \"%s - %s\" And Its Items", flyer.Merchant, flyer.Name)
            if err := store.AddRetailGroup(flyer); err != nil {
                return err
            }

            items, err := finder.FindFlyerItems(flyer.ID)
            if err != nil {
                return err
            }

            for _, item := range items {
                has, err := store.HasRetailGroupItem(item)
                if err != nil {
                    return err
                }
                if !has {
                    if err := store.AddRetailGroupItem(item); err != nil {
                        return err
                    }
                }
            }
        }
    }

    return nil
}

// FlyerIsValid replaces the previous RetailGroupIsValid. storage.Flyer is
// the new type, but its ValidFrom/ValidTo fields are still strings today,
// so the body is unchanged — just the type and name.
func FlyerIsValid(flyer storage.Flyer) (bool, error) {
    validFrom, err := internal.ParseDate(flyer.ValidFrom)
    if err != nil {
        return false, err
    }
    validTo, err := internal.ParseDate(flyer.ValidTo)
    if err != nil {
        return false, err
    }
    now := time.Now()
    if now.After(validFrom) && now.Before(validTo) {
        return true, nil
    }
    return false, nil
}
```

Notes on the reconciler change:

- The variable renames (`scrapeClient` → `finder`, `retailGroup` →
  `flyer`, `retailGroupItem` → `item`) are mechanical.
- The inline expiry walk stays. The persistence-abstraction Plan 1 will
  replace it with `store.PruneExpired(ctx, time.Now())` later.
- The storage method names stay the same — `JSONFileStorage` still has
  `AddRetailGroup`, `RemoveRetailGroup`, `HasRetailGroupItem`, etc. The
  parameter type changes (`*storage.JSONFileStorage`), but the methods are
  unchanged in this worktree.
- The `ctx context.Context` parameter is added for forward-compatibility.
  The current body does not use it (no cancellation, no timeout). When
  persistence Plan 2 lands and the storage methods themselves take `ctx`,
  the parameter is already in place.

### cmd/scrapeFlyers.go rework

```go
package cmd

import (
    "stfg/internal/flyerfinder"
    "stfg/internal/flyerfinder/flipp"
    "stfg/internal/reconciler/scrape"
    "stfg/internal/storage"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "go.uber.org/zap"
)

var scrapeFlyersCmd = &cobra.Command{
    Use:   "scrape-flyers [postal-code]",
    Short: "Parse and scrape flyers from grocers of choice",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        postalCode := args[0]

        finder := flipp.NewFinder()
        var f flyerfinder.FlyerFinder = finder

        store, err := storage.NewJSONFileStorage()
        if err != nil {
            zap.S().Error("Failed To Setup Storage System", err)
            return
        }

        whitelist := viper.GetStringSlice("fly_finder.whitelist")
        if len(whitelist) == 0 {
            zap.S().Warn("fly_finder.whitelist is empty; no flyers will match")
        }

        err = scrape.Reconcile(cmd.Context(), f, store, scrape.ScrapeReconcilerOptions{
            PostalCode:           postalCode,
            RetailGroupWhiteList: whitelist,
        })
        if err != nil {
            zap.S().Error("Error Scraping Flyers", err)
        }
    },
}

func init() {
    rootCmd.AddCommand(scrapeFlyersCmd)
}
```

### cmd/root.go — viper default

```go
// In initConfig() in cmd/root.go, add:
viper.SetDefault("fly_finder.whitelist", []string{
    "Superstore",
    "Thrify Foods",
    "Quality Foods",
    "Buy-Low Foods",
    "Country Grocer",
    "No Frills",
    "Pharmasave",
    "Shoppers Drug Mart",
    "Walmart",
    "Nesters Market",
    "Rexall",
})
```

`viper.SetDefault` only sets the value if the key is not already present
(from config file, env var, or flag). Users can override the whitelist in
`~/.stfg.yaml`:

```yaml
fly_finder:
  whitelist:
    - Superstore
    - Walmart
    - Costco
```

Env var override: `STFG_FLY_FINDER_WHITELIST` (viper auto-translates dots
to underscores and uppercases with the `STFG_` prefix because of
`viper.SetEnvPrefix("STFG")`).

### Sentinel / error changes

None. The flipp client returns errors with the existing style
(`fmt.Errorf("bad response: %d", res.StatusCode)`); the reconciler
propagates them. Sentinel errors like `storage.ErrNotFound` are
introduced by persistence Plan 1, not here.

### Tests to add

Per the testing directive on this project: every line of new or changed
code gets a test. The tests added by this plan:

| Symbol under test | Test name | What it asserts |
|---|---|---|
| `flyerfinder.FlyerFinder` interface | `TestFlyerFinder_InterfaceContract` | A small `mockFinder` struct implementing both methods compiles. `var _ flyerfinder.FlyerFinder = (*mockFinder)(nil)` is true. |
| `flipp.NewFinder` | `TestNewFinder_ReturnsClient` | `NewFinder()` returns `*flipp.Client` (concrete struct, not interface). |
| `flipp.Client.FindFlyers` — happy path | `TestFindFlyers_BuildsCanonicalFlyers` | Stub `httptest.Server` returning a Flipp-shaped flyers JSON. Call `FindFlyers("V5K0A1")`. Assert: returned slice contains `storage.Flyer` (not `RetailGroup`), with `ID`, `ValidFrom`, `ValidTo`, `Name`, `Merchant`, `Stores` populated. `Stores` is `[]storage.Store` (not `RetailGroupLocation`). |
| `flipp.Client.FindFlyers` — falls back to AvailableFrom/To | `TestFindFlyers_FallsBackToAvailableDates` | Flipp response has only `available_from` / `available_to` populated. Assert the resulting `storage.Flyer.ValidFrom` / `ValidTo` come from those fields. |
| `flipp.Client.FindFlyers` — skips nil dates | `TestFindFlyers_SkipsFlyersWithNoDates` | Flipp response has a flyer with both `valid_from` and `available_from` nil. Assert that flyer is not present in the returned slice. |
| `flipp.Client.FindFlyerItems` — happy path | `TestFindFlyerItems_BuildsCanonicalItems` | Stub HTTP. Assert returned slice is `[]storage.FlyerItem` with `FlyerID` set to the input parameter, `VideoURL` is `string` (empty when Flipp sends `*string` nil). |
| `scrape.Reconcile` — new signature | `TestReconcile_NewSignatureCompiles` | Calling `Reconcile(ctx, finder, store, opts)` with a `flyerfinder.FlyerFinder` typed value (not `*flipp.Client`) compiles and runs. |
| `scrape.Reconcile` — finder parameter is interface-typed | `TestReconcile_AcceptsAnyFlyerFinder` | A mock finder that returns no flyers triggers the "no flyers added" path without touching flipp. Confirms the reconciler doesn't import flipp. (Grep-style static check: `grep -L flipp internal/reconciler/scrape/*.go` returns zero matches.) |
| `scrape.FlyerIsValid` — renamed | `TestFlyerIsValid_HandlesStringDates` | Construct a `storage.Flyer` with `ValidFrom`/`ValidTo` as RFC3339 strings; assert `FlyerIsValid` returns true within the window, false outside. |
| `cmd/scrapeFlyers` — whitelist from viper | `TestScrapeFlyers_UsesViperWhitelist` | Set viper key `fly_finder.whitelist` to `["Superstore"]`. Invoke the cmd. Assert: the reconciler receives `ScrapeReconcilerOptions.RetailGroupWhiteList == ["Superstore"]`. (Test uses a captured value via a small injectable, OR uses viper's `Set` and a wrapper that exposes the option the reconciler received. See "Outstanding items" #2.) |
| `cmd/root.go` — default whitelist | `TestRootConfig_DefaultWhitelist` | With no `~/.stfg.yaml`, `viper.GetStringSlice("fly_finder.whitelist")` returns the 11-element default list. |

Helpers used by the tests:

- `mustFlyerJSON(t, flyers ...flipp.Flyer) []byte` — serialises Flipp DTOs to JSON for httptest responses.
- `mockFinder` — implements `flyerfinder.FlyerFinder` with controllable return values; lives in `internal/flyerfinder/flyerfinder_test.go` and is reused in `engine_test.go`.

Test cycle policy. Tests are produced by the test-writer agent and
reviewed by the test-reviewer agent. The cycle repeats until the reviewer
has no feedback or 5 cycles have elapsed.

## What gets added vs changed in this plan

### Added

- `internal/flyerfinder/flyerfinder.go`
- `internal/flyerfinder/flyerfinder_test.go`
- `internal/flyerfinder/flipp/client_test.go`
- `internal/reconciler/scrape/engine_test.go`

### Changed

- `internal/flipp/` → `internal/flyerfinder/flipp/`. **Move, not just
  re-typed.** All files (`client.go`, `types.go`, `utils.go`) move
  wholesale into the sub-package. The package's short name (`flipp`) is
  preserved; what changes is the import path callers use
  (`stfg/internal/flyerfinder/flipp` instead of `stfg/internal/flipp`).
- `internal/flyerfinder/flipp/client.go` — methods `FindFlyers` /
  `FindFlyerItems` replace `GetRetailGroups` / `GetRetailGroupItems`;
  returns `[]storage.Flyer` / `[]storage.FlyerItem`; constructs canonical
  records from Flipp DTOs at the boundary.
- `internal/flyerfinder/flipp/types.go` — `StoreLocation` renamed to
  `RetailGroupLocation`; `GetStoresNearByResponse` element type updated.
- `internal/reconciler/scrape/types.go` — `ScrapeClient`,
  `StorageClient`, `RetailGroup`, `RetailGroupItem`,
  `RetailGroupLocation` all deleted. `ScrapeReconcilerOptions` retained.
- `internal/reconciler/scrape/engine.go` — signature gains
  `ctx context.Context`; `finder` parameter typed as
  `flyerfinder.FlyerFinder`; `store` parameter typed as
  `*storage.JSONFileStorage`; variable renames; `RetailGroupIsValid`
  renamed to `FlyerIsValid` taking `storage.Flyer`.
- `cmd/scrapeFlyers.go` — `flipp.NewFinder()`; `var f flyerfinder.FlyerFinder = finder`; whitelist loaded via viper; `cmd.Context()` passed to `Reconcile`.
- `cmd/root.go` — `viper.SetDefault("fly_finder.whitelist", [...])`.

### Not changed in this plan

- `internal/storage/db.go` and `internal/storage/jsonfilestorage.go` —
  the dual-implementation mess stays until persistence Plan 2.
- `cmd/findDeals.go`, `cmd/addGrocery.go`, `cmd/listGroceries.go`,
  `cmd/removeGrocery.go`, `cmd/groceries.go` — none touch flipp.
- `internal/promptwriter/`, `internal/provider/`, `internal/models/` —
  not flipp-related.
- `internal/reconciler/scrape/engine.go` — the inline expiry walk stays.

## Expiry handling

This plan deliberately does NOT introduce `storage.PruneExpired`. The
current inline walk in `Reconcile`:

```go
// iterate over stored flyers and remove what's invalid
for _, flyer := range flyers {
    if !FlyerIsValid(flyer) {
        // remove the flyer and all its items
    }
}
```

stays exactly as it is. When persistence Plan 1 lands and
`storage.Storage.PruneExpired(ctx, now)` exists, this walk is replaced
by a single line. Until then, leaving it in place is the smallest diff
that achieves the FlyerFinder abstraction without spilling into storage
concerns.

## Future Work

### Future StoreCatalog interface

This plan does not introduce a `StoreCatalog` (or similar) interface for
regular-price observation at physical stores. The intent is for such an
abstraction to exist as a sibling to FlyerFinder, not a parent or
replacement of it.

Motivation: a Thrifty Foods flyer may advertise 10% off chicken, but
Superstore's regular chicken price after markup may already be lower.
Future work will need a way to look up the regular price at a given
store for a given item, and compare it against the flyer deal. The
`StoreCatalog` interface would own that lookup:

```
   ┌──────────────────────┐
   │  FlyerFinder         │  ← this plan
   │  "source of deals"   │
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │  StoreCatalog        │  ← future plan
   │  "regular prices"    │
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │  Comparator          │  ← future plan
   └──────────────────────┘
```

The two interfaces would not share a type. FlyerFinder returns
`[]storage.Flyer` (bounded-time, validity-windowed artifacts);
StoreCatalog would return e.g. `[]StorePriceQuote{ItemName, Price,
StoreID, StoreName, ObservedAt time.Time}` (current observations of
stable entities). No `ValidFrom`/`ValidTo` overlap; no shared struct
required.

The `storage.Store` type (a physical location: ID, Address, City,
Province, PostalCode) is unaffected by either future plan. It continues
to be a member of `storage.Flyer.Stores`. The future `StoreCatalog`
interface name does not collide with the `storage.Store` type because
the latter is a *location* (a value type) and the former is a *source*
(an interface). Different package, different role.

If `StoreCatalog` is eventually introduced, no change to FlyerFinder or
to this plan is required. The reconciler today doesn't know about
stores at all; `cmd/scrapeFlyers` doesn't know about stores; the
FlyerFinder interface doesn't know about stores. All clean separation.

Concretely, the future `StoreCatalog` would land at
`internal/flyerfinder/storecatalog/`, as a sibling sub-package to
`flipp/`:

```
internal/flyerfinder/
├── flyerfinder.go                # FlyerFinder interface
├── flyerfinder_test.go
├── flipp/                        # this plan: promotional flyer aggregator
│   ├── client.go
│   ├── types.go
│   └── ...
└── storecatalog/                 # future: regular-price observation
    ├── client.go                 # implements flyerfinder.StoreCatalog
    └── types.go
```

Whether `StoreCatalog` is its own top-level interface in
`internal/flyerfinder/flyerfinder.go` or a separate interface in a
sibling package (`internal/storecatalog/`) is a decision deferred to
that future plan. The point is that FlyerFinder is not the parent and
does not need to grow new methods to accommodate it.

## Acceptance criteria

1. `go build ./...` passes after the changes.
2. `flyerfinder.FlyerFinder` is the only flyer-source interface declared
   in the repo. `internal/reconciler/scrape/types.go` no longer declares
   `ScrapeClient` or `StorageClient`. Verified by
   `grep -rn "type .* interface" --include="*.go" internal/ cmd/` — the
   only flyer-source interface is `flyerfinder.FlyerFinder`.
3. `flipp.Client` is the only implementation of `FlyerFinder`. Verified
   by `grep -rn "flyerfinder.FlyerFinder" --include="*.go"` — the only
   consumer-side assignment is `var f flyerfinder.FlyerFinder = finder`
   in `cmd/scrapeFlyers.go` and the compile-time assertion in
   `internal/flyerfinder/flipp/client.go`.
4. `internal/reconciler/scrape/` does not import
   `internal/flyerfinder/flipp`. Verified by `grep -rn
   "stfg/internal/flyerfinder/flipp" --include="*.go" internal/reconciler/`.
5. The `FlyerFinder` interface returns `[]storage.Flyer` /
   `[]storage.FlyerItem` — no `scrape.RetailGroup*` types in the
   interface signature. Verified by `grep -rn "RetailGroup" --include="*.go" internal/flyerfinder/`.
6. The whitelist is loaded from viper. With no `~/.stfg.yaml`, running
   `stfg scrape-flyers V5K0A1` filters against the same 11 retailers as
   today. Verified by `TestRootConfig_DefaultWhitelist`.
7. `cmd/findDeals.go` is unchanged. Verified by `git diff` showing no
   changes to that file.

## Decisions (resolved before execution)

1. **Interface method names: `FindFlyers` / `FindFlyerItems`** — not
   `GetFlyers` / `GetFlyerItems`. The verb "find" matches the
   interface's name (`FlyerFinder`) and signals "look up and return
   what's available" rather than "fetch a specific resource by ID."
   The Flipp HTTP methods (`GetFlyers`, `GetFlyerItems`) keep their
   `Get*` names because they're literal HTTP verbs.
2. **No `Close()` on the interface.** The current flipp client doesn't
   have any long-lived resources to close. If a future implementation
   needs it, add later.
3. **Renaming `StoreLocation` → `RetailGroupLocation`.** For naming
   consistency with `Flyer` and `FlyerItem` DTOs in the same package.
   Slight risk: any external code importing `flipp.StoreLocation` would
   break. Inside this repo, only `internal/flyerfinder/flipp/client.go`
   (formerly `internal/flipp/client.go`) and
   `internal/reconciler/scrape/types.go` reference it. The latter is
   deleted in this plan.
4. **Variable rename `scrapeClient` → `finder` in `engine.go`.** The
   type is now an interface, not a concrete client. The old name
   implies `*flipp.Client`, which would mislead readers.
5. **Parameter type in `Reconcile` is `*storage.JSONFileStorage`, not
   `storage.Storage`.** The persistence-abstraction plans introduce
   `storage.Storage`, but this worktree does not land those plans.
   Typing as the concrete struct keeps the diff minimal and the call
   sites in `cmd/scrapeFlyers.go` work without modification beyond the
   type rename. When persistence Plan 2 lands, this becomes
   `storage.Storage` mechanically.

## Outstanding items for the implementer

These are non-blocking items the implementer must verify during
execution. None gate this plan's completion:

1. **Compile-time assertion placement.** The `var _ flyerfinder.FlyerFinder = (*Client)(nil)` lives in `internal/flyerfinder/flipp/client.go`, not in `internal/flyerfinder`. The assertion is co-located with the implementation it asserts on (Go idiom). It runs at package init and has zero runtime cost.

2. **Testing `cmd/scrapeFlyers.go`'s viper wiring.** Cobra commands are not naturally unit-testable because they wire their own dependencies. Two reasonable approaches:
   - (a) Extract a `runScrapeFlyers(ctx, finder flyerfinder.FlyerFinder, store *storage.JSONFileStorage, whitelist []string) error` helper that the cobra `Run` calls. Test the helper directly with an injected mock finder and mock store.
   - (b) Test the cobra command via `cmd.SetArgs(...)` and `cmd.Execute()` with a stub finder, capturing the options passed to the reconciler via a test-only seam.
   The implementer picks (a) or (b). (a) is the smaller, more idiomatic change.

3. **Unused `RetailGroupWhiteList` field rename in `ScrapeReconcilerOptions`.** The field is now misnamed (it operates on flyers, not retail groups). Renaming it to `FlyerWhiteList` is mechanical but is a public-API change. Keep the name as-is in this plan for minimal diff; rename as a follow-up if desired.

4. **The `internal.ParseDate` import in `engine.go` becomes dead if `FlyerIsValid` is moved into storage.** Today `FlyerIsValid` lives in `engine.go` and uses `internal.ParseDate`. Persistence Plan 1's `PruneExpired` does its own date parsing inside the storage layer, making `FlyerIsValid` (and its `ParseDate` import) redundant. The implementer does NOT move `FlyerIsValid` here — that's persistence Plan 1's job.

5. **`displayType` on `storage.FlyerItem`.** The Flipp DTO includes `DisplayType`; today `JSONFileStorage.AddRetailGroupItem` reads it but no consumer (including the promptwriter) reads it back. This plan keeps the field but does not propagate it through `FindFlyerItems`. If a follow-up wants the promptwriter to receive `DisplayType`, extend `flipp.Client.FindFlyerItems` to include `flippItem.DisplayType` in the returned `storage.FlyerItem`. Not in scope here.

6. **`Storage.Store` (physical location) naming.** Untouched by this plan. The future `StoreCatalog` interface name does not collide because Go's package qualification disambiguates and the two roles are different. See "Future Work."