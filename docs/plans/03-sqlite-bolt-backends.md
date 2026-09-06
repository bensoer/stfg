# Plan 3 — SQLite and BoltDB Backends

**Order:** 3 of 3
**Status:** Draft — DO NOT EXECUTE
**Depends on:** Plan 1 (`Storage` interface, `Options`, sentinel errors, `Open` factory). Plan 2 is recommended but not strictly required — Plan 3 can land against the Plan 1 contract before the JSON amalgam is finished; the JSON stub in Plan 1 just needs to compile.

## Goal

Add two new `Storage` implementations that satisfy the Plan 1 interface, located in the same cache directory the JSON backend uses:

1. **SQLite** — relational store, single file (`stfg.sqlite`), good fit for indexed lookups and ad-hoc queries.
2. **BoltDB** — embedded key-value store, single file (`stfg.bolt`), good fit for ordered scans and bucket-per-entity layout.

Both backends self-create their database file, schema (or buckets), and directory permissions on first use. The factory in Plan 1 routes `BackendSQLite` and `BackendBolt` to these.

## Ben-mcp-server / context7 / grepp guidance

- **ben-mcp-server:** no relevant standards. Proceeding with Go-idiomatic conventions.
- **context7:** will be queried during execution (this is a plan, not the execution) for the exact driver APIs (`modernc.org/sqlite` vs `mattn/go-sqlite3`, `go.etcd.io/bbolt`). For the plan document, recommend libraries and link to the query-during-execution step.

## Library choices (to be confirmed during execution via context7)

### SQLite — `modernc.org/sqlite`

Two main options:

| Driver | Pure Go? | CGO? | Notes |
|---|---|---|---|
| `github.com/mattn/go-sqlite3` | No | Yes | Most widely used; needs CGO. Conflicts with the project's existing CGO setup for llama.cpp. |
| `modernc.org/sqlite` | Yes | No | Pure-Go translation of SQLite. Drop-in `database/sql` driver. **Recommended** because the project already requires CGO for llama.cpp and adding a second CGO dep complicates the build. |

Recommendation: `modernc.org/sqlite`. It registers as a `database/sql` driver (`sqlite`), no build-tag gymnastics, no CGO.

### BoltDB — `go.etcd.io/bbolt`

`bbolt` is the maintained fork of `github.com/boltdb/bolt` (CoreOS archived the original). Pure Go, no CGO. Latest stable: bbolt v1.4.x.

Recommendation: `go.etcd.io/bbolt`.

## File layout after this plan

```
internal/storage/
├── storage.go              # interface, Options, errors, Open (Plan 1)
├── types.go                # canonical types (Plan 1)
├── json/
│   └── jsonfilestorage.go  # (Plan 2)
├── sqlite/
│   ├── sqlitestorage.go    # Open, Migrate, CRUD
│   └── migrate.go          # CREATE TABLE statements
└── bolt/
    ├── boltstorage.go      # Open, Migrate, CRUD
    └── migrate.go          # bucket creation
```

## Shared concerns across both backends

1. **Location.** Both files land in `opts.CacheDir` (defaulted from `storage.CacheDir()`). Names:
   - SQLite → `stfg.sqlite`
   - Bolt → `stfg.bolt`
   Both overridable via `Options.SQLiteFileName` / `Options.BoltFileName` for tests.
2. **Directory creation.** `Open(ctx, opts)` calls `os.MkdirAll(opts.CacheDir, 0o700)` before opening the file. If the dir already exists at a looser mode, do not chmod — surface as a warning in the log.
3. **File permissions.** `0o600` on the database file (owner read/write only). DB engines set this on create; for SQLite, `?_mode=ro` is irrelevant, but file mode is whatever the OS applies on `Open`. Verify with `os.Stat` after open and warn if wider than `0o600`.
4. **Locking.** SQLite uses its own file-locking; bbolt uses `flock` (BSD locks). No application-level mutex needed beyond what the driver provides.
5. **`Migrate(ctx)`** is the right place for first-use setup. Idempotent — safe to call on every `Open`.
6. **Concurrency.** Both drivers are safe for concurrent use within a single process; the `Storage` interface contract from Plan 1 says so explicitly.
7. **`Close()`.** SQLite: `db.Close()`. Bolt: `db.Close()`. Both must be called; the `Open` doc-comment says so.
8. **Error wrapping.** All "not found" cases return `fmt.Errorf("%w: ...", storage.ErrNotFound)`. `Unique constraint` violations on `Add*` return `fmt.Errorf("%w: ...", storage.ErrDuplicate)`.

## SQLite backend

### Schema (canonical, version 1)

```sql
-- Versioning row, lets us migrate later without breaking on-disk format.
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS groceries (
    name      TEXT PRIMARY KEY COLLATE NOCASE,   -- case-insensitive uniqueness
    embedding BLOB NOT NULL                      -- little-endian float32 slice
);

CREATE TABLE IF NOT EXISTS flyers (
    id         INTEGER PRIMARY KEY,
    valid_from TEXT NOT NULL,                    -- RFC3339, parseable by time.Parse(time.RFC3339, …)
    valid_to   TEXT NOT NULL,
    name       TEXT NOT NULL,
    merchant   TEXT NOT NULL,
    -- stores: 1-row-per-flyer with multiple stores serialised as JSON text
    stores     TEXT NOT NULL DEFAULT '[]'        -- JSON array of storage.Store
);

CREATE TABLE IF NOT EXISTS flyer_items (
    id         INTEGER PRIMARY KEY,
    flyer_id   INTEGER NOT NULL,
    name       TEXT NOT NULL,
    brand      TEXT,
    price      TEXT,
    image_url  TEXT,
    video_url  TEXT,
    FOREIGN KEY (flyer_id) REFERENCES flyers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_flyer_items_flyer_id ON flyer_items(flyer_id);
CREATE INDEX IF NOT EXISTS idx_flyers_valid_to      ON flyers(valid_to);
```

Notes:

- `embedding BLOB` — `[]float32` serialised as `binary.LittleEndian.PutUint32` per element. `embeddings` size is ~4 KB at 1024 dims, negligible.
- `name TEXT PRIMARY KEY COLLATE NOCASE` makes `HasGrocery` / `RemoveGrocery` case-insensitive without an extra query.
- `stores` column is JSON text rather than a third table — the number of stores per flyer is small (≤10), and the JSON blob is simpler. If we ever want to query "all flyers available at postal code V5K0A1" efficiently, promote to a `flyer_stores` table in v2.
- `ON DELETE CASCADE` removes items when a flyer is deleted at the SQL layer.

### `Migrate(ctx)` behaviour

- Read `schema_version.version`. If 0, run all `CREATE TABLE` / `CREATE INDEX` statements (they're all `IF NOT EXISTS`, so idempotent), then `INSERT INTO schema_version (version) VALUES (1)`.
- Wrap in a single transaction so a partial migration never lands.

### Driver registration

```go
import (
    _ "modernc.org/sqlite" // registers "sqlite" driver
    "database/sql"
)

db, err := sql.Open("sqlite", filepath.Join(opts.CacheDir, fileName) +
    "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)")
```

WAL mode: better concurrent read while a single writer is active. `foreign_keys=1`: required for `ON DELETE CASCADE` to actually fire in SQLite (off by default).

### `PruneExpired` SQL

```sql
DELETE FROM flyers WHERE valid_to < ?;  -- arg: now.Format(time.RFC3339)
```

Cascades to items via `ON DELETE CASCADE`.

### `AddGrocery` dedup

The `name` column's `PRIMARY KEY COLLATE NOCASE` makes duplicate adds a unique-constraint violation. Driver returns `sqlite3.Error{Code: sqlite3.CONSTRAINT}`. We detect via `errors.Is` / code match and wrap as `storage.ErrDuplicate`. **No load-then-save race.**

## BoltDB backend

### Bucket layout (canonical, version 1)

```
stfg.bolt
├── meta                     -- key="schema_version", value=int64(1)
├── groceries                -- key=grocery.Name (lower-cased), value=JSON(GroceryItem)
│                              -- Bucket enforces uniqueness by virtue of Bolt's key constraint.
├── flyers                   -- key=flyer.ID (int64 big-endian), value=JSON(Flyer)
├── flyer_items              -- nested bucket per flyer
│   ├── <flyer_id:BEint64>/
│   │   ├── <item_id:BEint64> → JSON(FlyerItem)
│   │   └── …
│   └── …
└── stores                   -- key=flyer.ID, value=JSON([]Store)  (kept separate to avoid
                                 rewriting the whole flyer on store-list changes)
```

Notes:

- Grocery names are normalised to `strings.ToLower` on insert and on lookup, so `HasGrocery` / `RemoveGrocery` are case-insensitive by construction.
- Item IDs use big-endian int64 encoding (`binary.BigEndian.PutUint64`) so lexicographic bucket iteration yields numeric order — important for `ListFlyerItems` consistency.
- A versioned `meta` bucket means future migrations can detect an old DB and refuse to open (or upgrade in place).

### `Migrate(ctx)` behaviour

```go
if err := db.Update(func(tx *bolt.Tx) error {
    if _, err := tx.CreateBucketIfNotExists([]byte("meta")); err != nil { return err }
    if _, err := tx.CreateBucketIfNotExists([]byte("groceries")); err != nil { return err }
    if _, err := tx.CreateBucketIfNotExists([]byte("flyers")); err != nil { return err }
    if _, err := tx.CreateBucketIfNotExists([]byte("flyer_items")); err != nil { return err }
    if _, err := tx.CreateBucketIfNotExists([]byte("stores")); err != nil { return err }
    // schema_version
    meta := tx.Bucket([]byte("meta"))
    if v := meta.Get([]byte("schema_version")); v == nil {
        return meta.Put([]byte("schema_version"), []byte{1})
    }
    return nil
}); err != nil { return err }
```

Idempotent. Wrapped in a single `db.Update` (write transaction).

### `PruneExpired` semantics

Bolt has no queries. The implementation is:

```go
return db.Update(func(tx *bolt.Tx) error {
    b := tx.Bucket([]byte("flyers"))
    itemsB := tx.Bucket([]byte("flyer_items"))
    nowBytes := []byte(now.Format(time.RFC3339))

    c := b.Cursor()
    var stale [][]byte
    for k, v := c.First(); k != nil; k, v = c.Next() {
        var f storage.Flyer
        if err := json.Unmarshal(v, &f); err != nil { return err }
        if f.ValidTo.Before(now) {
            stale = append(stale, append([]byte(nil), k...))
        }
    }
    for _, k := range stale {
        // Cascade: drop the items sub-bucket
        if ib := itemsB.Bucket(k); ib != nil { if err := itemsB.DeleteBucket(k); err != nil { return err } }
        if err := b.Delete(k); err != nil { return err }
    }
    _ = nowBytes
    return nil
})
```

The "gather keys first, then delete" pattern avoids mutating the bucket during iteration.

### `AddGrocery` dedup

Bolt's `Bucket.Put` overwrites unconditionally. To enforce uniqueness:

```go
existing := b.Get([]byte(strings.ToLower(item.Name)))
if existing != nil { return fmt.Errorf("%w: grocery %q", storage.ErrDuplicate, item.Name) }
return b.Put([]byte(strings.ToLower(item.Name)), jsonBytes)
```

There's a race window between `Get` and `Put`. Bolt serialises write transactions, so two `AddGrocery` calls in parallel serialise correctly. Read transactions don't block each other; only writes do.

## Interface mapping cheatsheet

| Plan 1 method | SQLite implementation | Bolt implementation |
|---|---|---|
| `Migrate(ctx)` | `CREATE TABLE IF NOT EXISTS …` in tx | `CreateBucketIfNotExists` in `db.Update` |
| `AddGrocery(ctx, item)` | `INSERT INTO groceries …` (PRIMARY KEY → duplicate) | `Bucket.Put` after `Bucket.Get` check |
| `RemoveGrocery(ctx, name)` | `DELETE FROM groceries WHERE name=? COLLATE NOCASE` | `Bucket.Delete(strings.ToLower(name))` |
| `ListGroceries(ctx)` | `SELECT name, embedding FROM groceries ORDER BY name` | iterate `groceries` bucket in key order |
| `HasGrocery(ctx, name)` | `SELECT 1 FROM groceries WHERE name=? COLLATE NOCASE LIMIT 1` | `Bucket.Get(strings.ToLower(name)) != nil` |
| `AddFlyer(ctx, flyer)` | `INSERT INTO flyers …` (PRIMARY KEY on `id` → duplicate) | `flyers` bucket; serialise stores to `stores` bucket in same tx |
| `RemoveFlyer(ctx, id)` | `DELETE FROM flyers WHERE id=?` (CASCADE → items) | delete `flyers[id]` + nested `flyer_items/<id>/` sub-bucket in same tx |
| `GetFlyer(ctx, id)` | `SELECT … FROM flyers WHERE id=?`, rehydrate stores from `stores` | `flyers[id]` + `stores[id]`; merge |
| `ListFlyers(ctx)` | `SELECT … FROM flyers ORDER BY id` | iterate `flyers` bucket, merge stores |
| `HasFlyer(ctx, id)` | `SELECT 1 FROM flyers WHERE id=? LIMIT 1` | `flyers[id] != nil` |
| `AddFlyerItem(ctx, item)` | `INSERT INTO flyer_items …` | `flyer_items/<flyer_id>/<item_id>` |
| `RemoveFlyerItem(ctx, flyerID, itemID)` | `DELETE FROM flyer_items WHERE flyer_id=? AND id=?` | delete that key |
| `ListFlyerItems(ctx, flyerID)` | `SELECT … FROM flyer_items WHERE flyer_id=? ORDER BY id` | iterate nested bucket |
| `HasFlyerItem(ctx, flyerID, itemID)` | `SELECT 1 FROM flyer_items WHERE flyer_id=? AND id=? LIMIT 1` | key existence in nested bucket |
| `PruneExpired(ctx, now)` | `DELETE FROM flyers WHERE valid_to < ?` (cascade) | gather stale keys, then delete flyer + items bucket |
| `Close()` | `db.Close()` | `db.Close()` |

## Store handling

Both backends store the `[]Store` separately from the `Flyer` row to keep the primary `flyers` row small and to avoid rewriting the whole flyer when stores change. The interface returns a fully-populated `Flyer{Stores: []Store{...}}` regardless.

- SQLite: `stores` table is `flyer_id INTEGER PRIMARY KEY, stores_json TEXT` — or as in the schema above, embedded in the `flyers` row. **Decision pending**: Plan 3 leans toward embedded JSON text in `flyers.stores` for simplicity; promote to a table if queries on stores become needed.
- Bolt: `stores` bucket, `key=flyerID → value=JSON([]Store)`.

If we change this during execution, the `Flyer` round-trip tests (in Plan 2's test file) are the contract that catches the change.

## Configuration surface

Add to `cmd/root.go` (Viper-bound):

```go
rootCmd.PersistentFlags().String("storage-backend", "json", "persistence backend: json|sqlite|bolt")
viper.BindPFlag("storage_backend", rootCmd.PersistentFlags().Lookup("storage-backend"))
viper.SetDefault("storage_backend", "json")
```

`cmd/groceries.go`'s `PersistentPreRunE` reads it:

```go
backend := storage.Backend(viper.GetString("storage_backend"))
store, err := storage.Open(ctx, storage.Options{
    Backend:  backend,
    CacheDir: dir,
})
```

Env var: `STFG_STORAGE_BACKEND`.

Default stays `json` so existing users are unaffected. Plans 2 + 3 are entirely opt-in for non-JSON.

## Dependencies to add (in `go.mod` during execution)

```
modernc.org/sqlite v1.34.x        # SQLite driver (pure Go)
go.etcd.io/bbolt v1.4.x           # BoltDB
```

(Exact versions to be confirmed via `context7` at execution time.)

## Tests to add

`internal/storage/sqlite/sqlitestorage_test.go`:

- Round-trip each CRUD method against a `t.TempDir()`.
- `PruneExpired` removes a stale flyer and its items.
- `AddGrocery` twice with the same name (different case) returns `ErrDuplicate`.
- Concurrent `AddGrocery` from many goroutines produces no duplicates and no torn writes (use the `-race` flag in CI).

`internal/storage/bolt/boltstorage_test.go`:

- Same suite, against Bolt.
- `PruneExpired` does not iterate while mutating (verify by inserting a flyer whose `ValidTo` is in the past, calling `PruneExpired`, and asserting both the flyer row and its items sub-bucket are gone).

Cross-backend parity test (a single test that runs against all registered backends):

```go
func TestStorageContract(t *testing.T, factory func(t *testing.T) storage.Storage) {
    // exercises every method in the Storage interface
}
```

Each backend's `TestMain` (or test helper) registers itself, so adding a fourth backend (Postgres, Badger, etc.) later only requires writing one factory function.

## Acceptance criteria

1. `go build ./...` passes with the new packages.
2. `storage.Open(ctx, Options{Backend: BackendSQLite, CacheDir: t.TempDir()})` returns a working `Storage`; same for `BackendBolt`.
3. Both backends pass the cross-backend parity test suite.
4. `Migrate(ctx)` is idempotent: calling it twice produces the same on-disk state.
5. Database files land in `<cache>/stfg.sqlite` and `<cache>/stfg.bolt` respectively (overridable).
6. Database files are created with mode `0o600` (owner only).
7. `find-deals` runs end-to-end against both new backends without changes to the command (after Plan 2's wiring).
8. Concurrent `AddGrocery` against each backend, with `go test -race`, reports no data races.
9. `STFG_STORAGE_BACKEND=sqlite ./bin/stfg scrape-flyers V5K0A1 && STFG_STORAGE_BACKEND=sqlite ./bin/stfg find-deals` produces correct output against a freshly-migrated DB.

## Risks

1. **modernc/sqlite compile time.** Pure-Go SQLite is slow to compile (translates the C source). First `go build` after adding the dep may take 30–60 seconds. Acceptable.
2. **BoltDB write amplification.** `AddFlyerItem` writes inside a nested bucket; thousands of small writes can cause bucket splits. Acceptable for the size of the data (one flyer ≈ hundreds of items at most).
3. **Schema migrations.** Plan 3 ships schema_version 1. If a v2 is ever needed, the migration path is `Migrate` checking the existing version and running the right DDL/transition. Plan 3 leaves the hook (`schema_version` table / `meta` bucket) but does not implement a migrator framework.
4. **Cross-backend data portability.** A user who switches from `json` to `sqlite` mid-life will not have their data migrated. This is out of scope for Plan 3; if desired, add a `storage.Migrate(from, to Storage)` helper in a future plan.

## Open questions for the owner before execution

1. **Driver choice.** `modernc.org/sqlite` (pure Go, recommended) vs `github.com/mattn/go-sqlite3` (CGO, smaller, faster). The project already requires CGO for llama.cpp, so adding a second CGO dep is the main argument against `mattn`. Confirm preference?
2. **Stores storage.** Embedded JSON text in the `flyers` row (simple, recommended) vs a dedicated `stores` table (queryable, more code). Confirm?
3. **grocery name case-insensitivity.** Bolt normalises via `strings.ToLower`; SQLite uses `COLLATE NOCASE`. Both are fine but they imply the index is on the lower-cased form (Bolt) vs the raw form with case-folding collation (SQLite). Confirm one approach across backends is acceptable (i.e. the API contract treats names case-insensitively and the implementation is private).
4. **`Close()` lifecycle.** Plan 1's `Storage.Close()` exists. Today JSON has nothing to close. After Plan 3, every backend must close — but the Cobra command currently doesn't track the store handle across subcommands. Plan 2 sets up the per-command handle; Plan 3 needs `Close()` called in `rootCmd.PersistentPostRunE` or equivalent. Confirm we wire that.
5. **Default backend at install time.** Stay `json` for backwards compatibility, or flip to `sqlite` for new users? Recommend `json` default for now.
6. **BoltDB maintenance.** Bolt files can grow if many deletes happen. `db.Shrink()` is an opt-in. Plan 3 calls `db.Shrink()` inside `Migrate` so users get free compaction; confirm acceptable, or schedule it elsewhere.
7. **`Embeddings` storage.** SQLite: `BLOB` of little-endian float32s. Bolt: stored as JSON in the grocery value. JSON serialises `[]float32` as a base64-encoded blob — same on-disk size as raw BLOB, slightly slower to read. If we wanted Bolt to match SQLite's raw BLOB storage, we'd use `encoding/gob` for the value. Recommend: Bolt uses `encoding/gob` for the grocery value (smaller, faster, still Go-native). Confirm.