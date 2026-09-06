package bolt

import (
	"context"
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

// newTestStorage creates a BoltStorage backed by a temporary directory.
func newTestStorage(t *testing.T) (*BoltStorage, context.Context) {
	t.Helper()
	ctx := context.Background()
	s, err := NewBolt(ctx, Options{CacheDir: t.TempDir(), BoltFileName: "test.bolt"})
	if err != nil {
		t.Fatalf("NewBolt failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, ctx
}

// TestStorageContract runs the cross-backend parity suite against bolt.
func TestStorageContract(t *testing.T) {
	parity.TestStorageContract(t, func(t *testing.T) storage.Storage {
		s, err := NewBolt(context.Background(), Options{CacheDir: t.TempDir(), BoltFileName: "test.bolt"})
		if err != nil {
			t.Fatalf("NewBolt: %v", err)
		}
		return s
	})
}

func TestNewBolt_CreatesDirAndFileWithOwnerOnlyPerms(t *testing.T) {
	// A non-existent nested path: the backend's own MkdirAll(0o700) creates it
	// (an already-existing dir like t.TempDir() at 0755 is intentionally left
	// untouched per the persistence plan).
	cacheDir := filepath.Join(t.TempDir(), "created-by-bolt")
	ctx := context.Background()

	fs, err := NewBolt(ctx, Options{CacheDir: cacheDir, BoltFileName: "test.bolt"})
	if err != nil {
		t.Fatalf("NewBolt failed: %v", err)
	}
	defer fs.Close()

	// Database file must exist and be no more permissive than 0o600.
	dbPath := filepath.Join(cacheDir, "test.bolt")
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
	// Bucket keys are the lowercased names, so iteration yields the same order
	// as sqlite's ORDER BY name: apple, Banana, Zebra.
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

func TestPruneExpired_DoesNotMutateWhileIterating(t *testing.T) {
	fs, ctx := newTestStorage(t)
	now := mustTime(t, "2025-06-15T12:00:00Z")

	// Interleave expired and current flyers so the gather-then-delete path is
	// exercised with keys both before and after skipped ones.
	if err := fs.AddFlyer(ctx, storage.Flyer{ID: 1, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), ValidTo: mustTime(t, "2025-06-01T00:00:00Z"), Name: "E1", Merchant: "M"}); err != nil {
		t.Fatalf("AddFlyer: %v", err)
	}
	if err := fs.AddFlyer(ctx, storage.Flyer{ID: 2, ValidFrom: mustTime(t, "2025-06-01T00:00:00Z"), ValidTo: mustTime(t, "2025-06-30T00:00:00Z"), Name: "C1", Merchant: "M"}); err != nil {
		t.Fatalf("AddFlyer: %v", err)
	}
	if err := fs.AddFlyer(ctx, storage.Flyer{ID: 3, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), ValidTo: mustTime(t, "2025-06-02T00:00:00Z"), Name: "E2", Merchant: "M"}); err != nil {
		t.Fatalf("AddFlyer: %v", err)
	}
	if err := fs.AddFlyer(ctx, storage.Flyer{ID: 4, ValidFrom: mustTime(t, "2025-06-20T00:00:00Z"), ValidTo: mustTime(t, "2025-06-30T00:00:00Z"), Name: "C2", Merchant: "M"}); err != nil {
		t.Fatalf("AddFlyer: %v", err)
	}

	if err := fs.PruneExpired(ctx, now); err != nil {
		t.Fatalf("PruneExpired: %v", err)
	}
	for _, id := range []int64{1, 3} {
		if has, _ := fs.HasFlyer(ctx, id); has {
			t.Errorf("expired flyer %d not removed", id)
		}
	}
	for _, id := range []int64{2, 4} {
		if has, _ := fs.HasFlyer(ctx, id); !has {
			t.Errorf("current flyer %d wrongly removed", id)
		}
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
