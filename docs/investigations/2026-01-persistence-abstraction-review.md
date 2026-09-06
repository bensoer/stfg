# Persistence Abstraction Review

**Status:** Investigation — verified against current code
**Scope:** How the stfg CLI persists and retrieves state to/from disk, and how (or whether) that persistence is abstracted.

## TL;DR

The belief that "there may be two different JSON file styles" is **confirmed**.
There are in fact **two parallel, fully independent persistence implementations** living in the same `internal/storage` package:

| Implementation | Style | Caller commands |
|---|---|---|
| `db.go` — flat package-level functions | Function-based, no struct | `listGroceries`, `removeGrocery`, `findDeals`, `groceries` (PersistentPreRunE) |
| `jsonfilestorage.go` — `JSONFileStorage` struct | Method-based, struct-valued | `addGrocery`, `scrapeFlyers` |

They share `CacheDir()` and the underlying file format (`json.Encoder` with 2-space indent), but they are **not** layered on top of one another and they disagree on the *on-disk schema* for the files they both touch.

There is **no abstraction layer** (no interface, no shared store) between callers and the filesystem today. Two read paths, two write paths, two schema conventions, no unified contract.

---

## 1. The single shared primitive

The only thing both implementations agree on is `storage.CacheDir()` (`internal/storage/utils.go`), which returns `<OSUserCacheDir>/stfg`, creating it on demand:

```go
const AppName = "stfg"

func CacheDir() (string, error) {
    base, err := os.UserCacheDir()
    ...
    dir := filepath.Join(base, AppName)
    os.MkdirAll(dir, 0755)
    return dir, nil
}
```

After that the two paths diverge.

## 2. Implementation A — `db.go` (flat functions)

**File:** `internal/storage/db.go`
**API surface used by callers:** `storage.LoadGroceries`, `storage.SaveGroceries`, `storage.LoadAllFlyers`, `storage.LoadFlyerItems`, `storage.EnsureGroceryFileExists`.

### Files it manages

| Filename | Function | Schema |
|---|---|---|
| `groceries.json` | `GroceryFile()` → `SaveGroceries([]string)` | `[]string` — bare names, **no embeddings** |
| `flyers_index.json` | `FlyersIndexFile()` | `[]Flyer` (`storage.Flyer`) |
| `flyer_<id>.json` | `FlyerFileName(flyerID)` | `[]FlyerItem` (`storage.FlyerItem`) |

### Caller usage

- `cmd/listGroceries.go` → `storage.LoadGroceries()` → prints as strings
- `cmd/removeGrocery.go` → `storage.LoadGroceries()` + `storage.SaveGroceries([]string)` → matches names case-insensitively
- `cmd/findDeals.go` → `storage.LoadAllFlyers()` + `storage.LoadFlyerItems(flyer.ID)`
- `cmd/groceries.go` PersistentPreRunE → `storage.EnsureGroceryFileExists()` (seeds the file as `[]string`)

### Notable details

- `EnsureGroceryFileExists()` writes `[]string{}` as the seed.
- `LoadGroceries` returns `[]string`.
- `RemoveInvalidFlyers` exists but **is never called** by any current command (grep shows zero callers).
- `SaveFlyer(flyer Flyer)` mutates the index in-place and rewrites the whole file — a de-dup-by-ID pattern that is also used (with slight differences) by the other implementation.
- `DeleteFlyerItems` is called by `RemoveInvalidFlyers`, but again only by that dead helper.

## 3. Implementation B — `jsonfilestorage.go` (struct + methods)

**File:** `internal/storage/jsonfilestorage.go`
**API surface used by callers:** `storage.NewJSONFileStorage()` returning a `*JSONFileStorage`, then methods on it.

### Files it manages

| Filename | Constant | Schema |
|---|---|---|
| `flyers_index.json` | `RetailGroupIndexFileName` | `[]scrape.RetailGroup` — but seeded as `[]string{}` ⚠️ |
| `groceries.json` | `GroceryFileName` | `[]GroceryItem` (with embeddings) — but seeded as such correctly |
| `flyer_<id>.json` | `retailGroupFileName(flyerID)` | `[]scrape.RetailGroupItem` |

### Constructor

```go
func NewJSONFileStorage() (*JSONFileStorage, error) {
    // ensure flyers_index.json — seeds []string{}   ⚠️ wrong schema
    // ensure groceries.json   — seeds []GroceryItem{}
    return &JSONFileStorage{}, nil
}
```

### Methods (used by callers)

- `GetAllGroceries`, `AddGrocery`, `RemoveGrocery` — for grocery items with embeddings
- `AddRetailGroup`, `RemoveRetailGroup`, `GetRetailGroup`, `GetAllRetailGroups`, `HasRetailGroup`
- `AddRetailGroupItem`, `RemoveRetailGroupItem`, `GetRetailGroupItems`, `HasRetailGroupItem`

### Notable details

- The constructor `NewJSONFileStorage` initializes `flyers_index.json` with `[]string{}` even though `AddRetailGroup` / `GetAllRetailGroups` read it back into `[]scrape.RetailGroup`. **The seed type and the read type do not match.** The first call that reads the index after a fresh install will fail to deserialize the seeded JSON.
- `groceries.json` is correctly seeded as `[]GroceryItem{}`.
- Implements the `scrape.StorageClient` interface (used by the reconciler).
- `loadJSON` / `saveJSON` are private methods on the struct. `db.go` defines its own `LoadJSON` / `SaveJSON` as **package-level** functions with identical bodies. They are duplicated, not shared.
- `RemoveRetailGroup` walks the items list and calls `RemoveRetailGroupItem` for each, but **discards the error** from each call (`j.RemoveRetailGroupItem(rtgi)` — return value ignored).

## 4. The "two JSON file styles" — verified

Both implementations write to the **same filenames** but expect **different schemas**, and both initialize the empty file with **the wrong type** in at least one case:

### `groceries.json`

| | Schema it writes | Schema it reads back |
|---|---|---|
| `db.go` | `[]string` | `[]string` ✅ |
| `jsonfilestorage.go` | `[]GroceryItem` (with `embedding`) | `[]GroceryItem` ✅ |

The two styles are **incompatible** — a file written by `add-grocery` (struct form) cannot be read by `list-groceries` or `remove-grocery` (string form), and vice versa. In practice the current first-run ordering (groceries command seeds as `[]string`, then `add-grocery` re-writes as `[]GroceryItem`) means the file will eventually be one or the other depending on which subcommand ran last.

### `flyers_index.json`

| | Schema it writes | Schema it reads back | Seed type |
|---|---|---|---|
| `db.go` | `[]storage.Flyer` | `[]storage.Flyer` ✅ | `[]Flyer{}` ✅ |
| `jsonfilestorage.go` | `[]scrape.RetailGroup` | `[]scrape.RetailGroup` ✅ | `[]string{}` ❌ |

Both styles use the same filename and write structurally similar (but **not identical**) records:

```go
// storage.Flyer (db.go's model)
type Flyer struct {
    ID int64 `json:"id"`
    ValidFrom string `json:"valid_from"`   // snake_case
    ValidTo string `json:"valid_to"`
    Name string `json:"name"`
    Merchant string `json:"merchant"`
    Stores []Store `json:"stores"`         // spelled "stores"
    ...
}

// scrape.RetailGroup (jsonfilestorage.go's model)
type RetailGroup struct {
    ID int64 `json:"id"`
    ValidFrom string `json:"validFrom,"`  // camelCase, plus a stray comma
    ValidTo string `json:"validTo"`
    Name string `json:"name"`
    Merchant string `json:"merchant"`
    Locations []RetailGroupLocation `json:"location"`  // singular "location", plural field
    ...
}
```

Even if both implementations wrote valid JSON, the field-name conventions and types are different enough that data produced by one would not round-trip cleanly through the other.

### `flyer_<id>.json`

- `db.go` writes `[]storage.FlyerItem` (includes `DisplayType`, `flyer_id`, `AvailableTo`, spatial coordinates).
- `jsonfilestorage.go` writes `[]scrape.RetailGroupItem` (no display type, `RetailGroupId` instead of `FlyerID`).

Same filename pattern (`flyer_%d.json`), different schemas.

## 5. Caller-coverage matrix

| Command | Storage path used | Files it reads | Files it writes |
|---|---|---|---|
| `groceries add-grocery` | `JSONFileStorage` | `groceries.json` | `groceries.json` |
| `groceries list-groceries` | flat `db.go` | `groceries.json` | — |
| `groceries remove-grocery` | flat `db.go` | `groceries.json` | `groceries.json` |
| `scrape-flyers` | `JSONFileStorage` | `flyers_index.json`, `flyer_<id>.json` | `flyers_index.json`, `flyer_<id>.json` |
| `find-deals` | flat `db.go` | `flyers_index.json`, `flyer_<id>.json`, `groceries.json` | — |

The grocery subcommands are split across both implementations in a way that **guarantees schema drift** between write/read:
- `add-grocery` writes the struct-with-embedding form.
- `list-groceries` reads the bare-string form.
- `remove-grocery` reads and writes the bare-string form.

If a user runs `add-grocery` first, then `list-groceries`, the list command will fail to decode the JSON into `[]string`.

## 6. Duplication and dead code

- `LoadJSON` / `SaveJSON` (package-level in `db.go`) and `loadJSON` / `saveJSON` (methods on `JSONFileStorage` in `jsonfilestorage.go`) are byte-for-byte identical bodies (file creation, 2-space indent, read+unmarshal).
- `FlyerFileName(flyerID int64)` and `retailGroupFileName(flyerID int64)` are the same `fmt.Sprintf("flyer_%d.json", flyerID)` formula.
- `FlyersIndexFile()` / `RetailGroupIndexFileName` resolve to the same literal `"flyers_index.json"`.
- `GroceryFile()` / `GroceryFileName` resolve to the same literal `"groceries.json"`.
- `RemoveInvalidFlyers` in `db.go` is unreachable from any command — dead code.
- `RetailGroupNotFound` error sentinel is declared in `types.go` but **only used by `jsonfilestorage.go`** — `db.go` returns plain `fmt.Errorf("flyer with ID %d not found", ...)` for the same not-found condition. Two different error styles for the same situation.

## 7. Summary of gaps for an abstraction pass

If the goal is a single, unified persistence layer, the following have to be reconciled:

1. **One type per file.** Pick a single grocery record type, a single flyer-record type, and a single flyer-item type. Right now there are two of each (`GroceryItem` vs `string`, `Flyer` vs `RetailGroup`, `FlyerItem` vs `RetailGroupItem`).
2. **One constructor / one handle.** Callers should obtain a storage handle through a single factory. `NewJSONFileStorage()` is already that handle, but `db.go`'s package-level functions bypass it.
3. **One error contract.** `RetailGroupNotFound` exists but `db.go`'s `LoadFlyer` returns `fmt.Errorf`. Pick one.
4. **Seed files with the correct empty type.** `NewJSONFileStorage` seeds `flyers_index.json` as `[]string{}` even though every consumer expects `[]scrape.RetailGroup{}`. Trivial to fix, currently broken.
5. **Deduplicate the JSON I/O.** `SaveJSON`/`LoadJSON` (package) and `saveJSON`/`loadJSON` (method) are the same code. Move them to one private helper.
6. **Decide what to do with `RemoveInvalidFlyers`.** It is unused and overlaps with the reconciler's own expiry handling (`internal/reconciler/scrape/engine.go` calls `RemoveRetailGroup` itself).
7. **Define a `Storage` interface** that both `cmd` and the `scrape` reconciler consume. The reconciler already declares `StorageClient` (`internal/reconciler/scrape/types.go`) — `JSONFileStorage` implements it; the flat `db.go` functions do not, which is why they are not used by the reconciler.
8. **Surface a `RemoveGrocery(name string)` API** that does the case-insensitive matching. Today it is duplicated across `cmd/addGrocery.go` (no — it does the dup check, but doesn't remove) and `cmd/removeGrocery.go` (does the actual remove on strings), each re-implementing the lookup.
9. **Fix the silently-discarded errors** in `RemoveRetailGroup` and `RemoveRetailGroupItem` callers.

---

## Appendix A — file / schema cross-reference

```
internal/storage/
├── db.go                    — flat functions; grocery = []string, flyer = []Flyer
├── jsonfilestorage.go       — struct methods; grocery = []GroceryItem, flyer = []RetailGroup
├── types.go                 — shared types: Store, Flyer, FlyerItem, GroceryItem, RetailGroupNotFound
└── utils.go                 — CacheDir() (only shared helper)

internal/reconciler/scrape/
├── engine.go                — uses scrape.StorageClient interface
└── types.go                 — defines StorageClient + RetailGroup / RetailGroupItem (camelCase JSON)

cmd/
├── groceries.go             — uses db.go (EnsureGroceryFileExists)
├── addGrocery.go            — uses JSONFileStorage
├── listGroceries.go         — uses db.go (LoadGroceries) — expects []string
├── removeGrocery.go         — uses db.go (LoadGroceries/SaveGroceries) — expects []string
├── scrapeFlyers.go          — uses JSONFileStorage
└── findDeals.go             — uses db.go (LoadAllFlyers/LoadFlyerItems)
```

## Appendix B — verifications performed

- `grep -rn "os.(Open|Create|ReadFile|WriteFile)|encoding/json|json\.(Marshal|Unmarshal|NewEncoder|NewDecoder)" --include="*.go"` — confirmed all persistence I/O lives in `internal/storage/{db.go,jsonfilestorage.go,utils.go}` plus the unrelated log file in `cmd/root.go` and GGUF download in `internal/models/resolver.go`.
- `grep -rn "storage\." --include="*.go"` — listed every caller of `storage.*` and confirmed which API surface each command uses.
- Read each of the seven `cmd/*.go` files, the four `internal/storage/*.go` files, both `internal/reconciler/scrape/*.go` files, and `internal/flipp/{client,types}.go` for context.
- Did **not** run `go build` or `go test`; this is a static review.