package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"stfg/internal/storage"
	"stfg/internal/storage/parity"
)

// mustTime parses an RFC3339 string or fails the test.
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", s, err)
	}
	return tm
}

// mustEmbed returns a deterministic float32 slice of the given dimension.
func mustEmbed(t *testing.T, dim int) []float32 {
	t.Helper()
	if dim <= 0 {
		return nil
	}
	out := make([]float32, dim)
	for i := range out {
		out[i] = float32(i) * 0.5
	}
	return out
}

// sliceEqualFloat32 reports whether two float32 slices are equal.
func sliceEqualFloat32(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// newTestStorage creates a SQLiteStorage backed by a temporary directory.
func newTestStorage(t *testing.T) (*SQLiteStorage, context.Context) {
	t.Helper()
	ctx := context.Background()
	s, err := NewSQLite(ctx, Options{CacheDir: t.TempDir(), SQLiteFileName: "test.sqlite"})
	if err != nil {
		t.Fatalf("NewSQLite failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, ctx
}

// TestStorageContract runs the cross-backend parity suite against sqlite.
func TestStorageContract(t *testing.T) {
	parity.TestStorageContract(t, func(t *testing.T) storage.Storage {
		s, err := NewSQLite(context.Background(), Options{CacheDir: t.TempDir(), SQLiteFileName: "test.sqlite"})
		if err != nil {
			t.Fatalf("NewSQLite: %v", err)
		}
		return s
	})
}

func TestNewSQLite_CreatesDirAndFileWithOwnerOnlyPerms(t *testing.T) {
	// A non-existent nested path: the backend's own MkdirAll(0o700) creates it
	// (an already-existing dir like t.TempDir() at 0755 is intentionally left
	// untouched per the persistence plan).
	cacheDir := filepath.Join(t.TempDir(), "created-by-sqlite")
	ctx := context.Background()

	fs, err := NewSQLite(ctx, Options{CacheDir: cacheDir, SQLiteFileName: "test.sqlite"})
	if err != nil {
		t.Fatalf("NewSQLite failed: %v", err)
	}
	defer fs.Close()

	// Database file must exist and be no more permissive than 0o600.
	dbPath := filepath.Join(cacheDir, "test.sqlite")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("db file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("db file mode %o is wider than 0o600 (group/other bits set)", perm)
	}
	// Cache dir created by the backend must be owner-only as well.
	dirInfo, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("cache dir stat: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("cache dir mode %o is wider than 0o700", perm)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add data so a destructive re-migrate would be caught.
	if err := fs.AddGrocery(ctx, storage.GroceryItem{Name: "milk", Embedding: mustEmbed(t, 3)}); err != nil {
		t.Fatalf("AddGrocery: %v", err)
	}
	if err := fs.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := fs.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	gs, err := fs.ListGroceries(ctx)
	if err != nil || len(gs) != 1 || gs[0].Name != "milk" {
		t.Fatalf("data lost after re-migrate: %+v err=%v", gs, err)
	}
}

// A database created by an older schema version (no display_name column) must
// be upgraded in place by Migrate, with display_name backfilled from name.
func TestMigrate_LegacyDBWithoutDisplayName(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "legacy.sqlite")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	stmts := []string{
		`CREATE TABLE schema_version (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')));`,
		`INSERT INTO schema_version (version) VALUES (1);`,
		`CREATE TABLE groceries (name TEXT PRIMARY KEY, embedding BLOB NOT NULL);`,
		`INSERT INTO groceries (name, embedding) VALUES ('milk', x'00000000');`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("legacy setup %q: %v", stmt, err)
		}
	}
	db.Close()

	ctx := context.Background()
	fs, err := NewSQLite(ctx, Options{CacheDir: tmpDir, SQLiteFileName: "legacy.sqlite"})
	if err != nil {
		t.Fatalf("NewSQLite on legacy db: %v", err)
	}
	defer fs.Close()

	gs, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries after upgrade: %v", err)
	}
	if len(gs) != 1 || gs[0].Name != "milk" {
		t.Fatalf("legacy row name = %q (want milk), rows=%d", gs[0].Name, len(gs))
	}
	// A fresh insert must work: the display_name column now exists with a
	// UNIQUE-enforced (name) constraint backed by Go-side lowercasing.
	if err := fs.AddGrocery(ctx, storage.GroceryItem{Name: "Eggs"}); err != nil {
		t.Fatalf("AddGrocery after upgrade: %v", err)
	}
	gs, _ = fs.ListGroceries(ctx)
	if len(gs) != 2 {
		t.Fatalf("expected 2 groceries after add, got %d", len(gs))
	}
	// Re-migrate must be a no-op.
	if err := fs.Migrate(ctx); err != nil {
		t.Fatalf("re-Migrate: %v", err)
	}
}

func TestListGroceries_OrderedByLowercasedNameAndPreservesDisplayCase(t *testing.T) {
	fs, ctx := newTestStorage(t)

	for _, name := range []string{"Banana", "apple", "Zebra"} {
		if err := fs.AddGrocery(ctx, storage.GroceryItem{Name: name}); err != nil {
			t.Fatalf("AddGrocery %s: %v", name, err)
		}
	}

	gs, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries: %v", err)
	}
	if len(gs) != 3 {
		t.Fatalf("expected 3 groceries, got %d", len(gs))
	}
	// Ordering is by the lowercased name key: apple, Banana, Zebra.
	want := []string{"apple", "Banana", "Zebra"}
	for i := range want {
		if gs[i].Name != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full: %+v)", i, gs[i].Name, want[i], gs)
		}
	}
}

func TestListFlyers_OrderedByID(t *testing.T) {
	fs, ctx := newTestStorage(t)

	now := mustTime(t, "2025-01-01T00:00:00Z")
	for _, id := range []int64{3, 1, 2} {
		if err := fs.AddFlyer(ctx, storage.Flyer{ID: id, ValidFrom: now, Name: fmt.Sprintf("F%d", id), Merchant: "M"}); err != nil {
			t.Fatalf("AddFlyer %d: %v", id, err)
		}
	}

	flyers, err := fs.ListFlyers(ctx)
	if err != nil {
		t.Fatalf("ListFlyers: %v", err)
	}
	if len(flyers) != 3 || flyers[0].ID != 1 || flyers[1].ID != 2 || flyers[2].ID != 3 {
		t.Fatalf("flyers not ordered by ID: %+v", flyers)
	}
}

func TestListFlyerItems_OrderedByID(t *testing.T) {
	fs, ctx := newTestStorage(t)

	if err := fs.AddFlyer(ctx, storage.Flyer{ID: 10, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "F", Merchant: "M"}); err != nil {
		t.Fatalf("AddFlyer: %v", err)
	}
	for _, id := range []int64{3, 1, 2} {
		if err := fs.AddFlyerItem(ctx, storage.FlyerItem{ID: id, FlyerID: 10, Name: fmt.Sprintf("i%d", id)}); err != nil {
			t.Fatalf("AddFlyerItem %d: %v", id, err)
		}
	}

	items, err := fs.ListFlyerItems(ctx, 10)
	if err != nil {
		t.Fatalf("ListFlyerItems: %v", err)
	}
	if len(items) != 3 || items[0].ID != 1 || items[1].ID != 2 || items[2].ID != 3 {
		t.Fatalf("items not ordered by ID: %+v", items)
	}
}

func TestAddFlyerItem_UnknownFlyer_ErrNotFound(t *testing.T) {
	fs, ctx := newTestStorage(t)
	err := fs.AddFlyerItem(ctx, storage.FlyerItem{ID: 1, FlyerID: 999, Name: "x"})
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestRemoveGrocery_CaseInsensitive(t *testing.T) {
	fs, ctx := newTestStorage(t)
	if err := fs.AddGrocery(ctx, storage.GroceryItem{Name: "MiLk"}); err != nil {
		t.Fatalf("AddGrocery: %v", err)
	}
	if err := fs.RemoveGrocery(ctx, "milk"); err != nil {
		t.Fatalf("RemoveGrocery: %v", err)
	}
	has, _ := fs.HasGrocery(ctx, "MILK")
	if has {
		t.Fatal("grocery still present after case-insensitive remove")
	}
}

func TestRemoveFlyer_Unknown_ErrNotFound(t *testing.T) {
	fs, ctx := newTestStorage(t)
	err := fs.RemoveFlyer(ctx, 42)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestGetFlyer_ReturnsCopy(t *testing.T) {
	fs, ctx := newTestStorage(t)
	f := storage.Flyer{
		ID: 1, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "Flyer", Merchant: "M",
		Stores: []storage.Store{{ID: 1, PostalCode: "A1A 1A1"}},
	}
	if err := fs.AddFlyer(ctx, f); err != nil {
		t.Fatalf("AddFlyer: %v", err)
	}
	got, err := fs.GetFlyer(ctx, 1)
	if err != nil {
		t.Fatalf("GetFlyer: %v", err)
	}
	// Mutating the returned pointer must not affect subsequent reads.
	got.Name = "Mutated"
	got.Stores[0].PostalCode = "ZZZ"
	again, err := fs.GetFlyer(ctx, 1)
	if err != nil {
		t.Fatalf("second GetFlyer: %v", err)
	}
	if again.Name != "Flyer" || again.Stores[0].PostalCode != "A1A 1A1" {
		t.Fatalf("GetFlyer returned shared state: %+v", again)
	}
}

func TestPruneExpired_SubSecond_LateNow(t *testing.T) {
	fs, ctx := newTestStorage(t)
	validTo, _ := time.Parse(time.RFC3339Nano, "2025-03-01T10:00:00.000Z")
	now, _ := time.Parse(time.RFC3339Nano, "2025-03-01T10:00:00.500Z")
	if err := fs.AddFlyer(ctx, storage.Flyer{ID: 1, ValidFrom: validTo.Add(-time.Hour), ValidTo: validTo, Name: "F", Merchant: "M"}); err != nil {
		t.Fatalf("AddFlyer: %v", err)
	}
	if err := fs.PruneExpired(ctx, now); err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	has, _ := fs.HasFlyer(ctx, 1)
	if has {
		t.Fatal("sub-second-expired flyer not pruned")
	}
}
