// Package parity hosts a single cross-backend storage contract test. Each
// backend's test package calls TestStorageContract with a factory that builds
// a fresh, empty backend in a temp dir, so every Storage implementation is
// verified against the same behavioural contract.
//
// The contract test is deliberately ordering-agnostic where the interface does
// not guarantee ordering: ListGroceries/ListFlyers/ListFlyerItems ordering is
// backend-specific and is asserted in each backend's own test suite.
package parity

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"stfg/internal/storage"
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

// equalFloat32 reports element-wise equality of two float32 slices.
func equalFloat32(a, b []float32) bool {
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

// newStore is a convenience for subtests: it builds a fresh backend and
// registers a cleanup that closes it.
func newStore(t *testing.T, factory func(*testing.T) storage.Storage) storage.Storage {
	t.Helper()
	s := factory(t)
	t.Cleanup(func() { s.Close() })
	return s
}

// TestStorageContract runs every method of storage.Storage against a fresh
// instance returned by factory. factory must return a new, empty backend; it
// is called once per subtest so subtests are fully isolated.
func TestStorageContract(t *testing.T, factory func(*testing.T) storage.Storage) {
	t.Helper()
	ctx := context.Background()

	t.Run("Migrate_Idempotent", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.Migrate(ctx); err != nil {
			t.Fatalf("first Migrate: %v", err)
		}
		if err := s.Migrate(ctx); err != nil {
			t.Fatalf("second Migrate: %v", err)
		}
		// Store stays usable after re-migrate.
		if err := s.AddGrocery(ctx, storage.GroceryItem{Name: "milk"}); err != nil {
			t.Fatalf("AddGrocery after re-migrate: %v", err)
		}
	})

	t.Run("EmptyLists_NonNil", func(t *testing.T) {
		s := newStore(t, factory)
		gs, err := s.ListGroceries(ctx)
		if err != nil || gs == nil || len(gs) != 0 {
			t.Fatalf("ListGroceries = %v (nil=%v), err=%v; want non-nil empty", gs, gs == nil, err)
		}
		fs, err := s.ListFlyers(ctx)
		if err != nil || fs == nil || len(fs) != 0 {
			t.Fatalf("ListFlyers = %v (nil=%v), err=%v; want non-nil empty", fs, fs == nil, err)
		}
		is, err := s.ListFlyerItems(ctx, 1)
		if err != nil || is == nil || len(is) != 0 {
			t.Fatalf("ListFlyerItems = %v (nil=%v), err=%v; want non-nil empty", is, is == nil, err)
		}
	})

	t.Run("Groceries_RoundTrip", func(t *testing.T) {
		s := newStore(t, factory)
		item := storage.GroceryItem{Name: "milk", Embedding: mustEmbed(t, 5)}
		if err := s.AddGrocery(ctx, item); err != nil {
			t.Fatalf("AddGrocery: %v", err)
		}
		gs, err := s.ListGroceries(ctx)
		if err != nil {
			t.Fatalf("ListGroceries: %v", err)
		}
		if len(gs) != 1 || gs[0].Name != "milk" || !equalFloat32(gs[0].Embedding, item.Embedding) {
			t.Fatalf("round-trip mismatch: %+v", gs)
		}
		has, err := s.HasGrocery(ctx, "milk")
		if err != nil || !has {
			t.Fatalf("HasGrocery(milk) = %v, err=%v", has, err)
		}
		if err := s.RemoveGrocery(ctx, "MILK"); err != nil {
			t.Fatalf("RemoveGrocery(MILK): %v", err)
		}
		gs, _ = s.ListGroceries(ctx)
		if len(gs) != 0 {
			t.Fatalf("expected empty after remove, got %d", len(gs))
		}
	})

	t.Run("Groceries_CaseInsensitive_Duplicate", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.AddGrocery(ctx, storage.GroceryItem{Name: "Milk"}); err != nil {
			t.Fatalf("AddGrocery(Milk): %v", err)
		}
		// Duplicate with different case must be rejected.
		err := s.AddGrocery(ctx, storage.GroceryItem{Name: "milk"})
		if !errors.Is(err, storage.ErrDuplicate) {
			t.Fatalf("duplicate add: got %v, want ErrDuplicate", err)
		}
		// List must report the original display case.
		gs, _ := s.ListGroceries(ctx)
		if len(gs) != 1 || gs[0].Name != "Milk" {
			t.Fatalf("display case lost: %+v", gs)
		}
	})

	t.Run("Groceries_Unicode_CaseInsensitive", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.AddGrocery(ctx, storage.GroceryItem{Name: "CAFÉ"}); err != nil {
			t.Fatalf("AddGrocery(CAFÉ): %v", err)
		}
		err := s.AddGrocery(ctx, storage.GroceryItem{Name: "café"})
		if !errors.Is(err, storage.ErrDuplicate) {
			t.Fatalf("Unicode duplicate add: got %v, want ErrDuplicate", err)
		}
		has, err := s.HasGrocery(ctx, "CaFé")
		if err != nil || !has {
			t.Fatalf("HasGrocery(CaFé) = %v, err=%v", has, err)
		}
		gs, _ := s.ListGroceries(ctx)
		if len(gs) != 1 || gs[0].Name != "CAFÉ" {
			t.Fatalf("original display case not preserved: %+v", gs)
		}
	})

	t.Run("Groceries_Unicode_CaseFold_FinalSigma", func(t *testing.T) {
		s := newStore(t, factory)
		// All three backends key groceries by strings.ToLower (via
		// storage.NormalizeName). Under ToLower, "Σ" (U+03A3) maps to "σ"
		// (U+03C3) and "ς" (U+03C2) maps to "ς" (U+03C2) — distinct code
		// points, so both are accepted as separate items. strings.EqualFold
		// would treat them as the same (case-fold maps both final-sigma and
		// sigma to σ), which is why the backends were unified on ToLower.
		for _, name := range []string{"Σ", "ς"} {
			if err := s.AddGrocery(ctx, storage.GroceryItem{Name: name}); err != nil {
				t.Fatalf("AddGrocery(%s): %v", name, err)
			}
		}
		for _, name := range []string{"Σ", "ς"} {
			has, err := s.HasGrocery(ctx, name)
			if err != nil {
				t.Fatalf("HasGrocery(%s): %v", name, err)
			}
			if !has {
				t.Fatalf("HasGrocery(%s) = false; want true", name)
			}
		}
		gs, err := s.ListGroceries(ctx)
		if err != nil {
			t.Fatalf("ListGroceries: %v", err)
		}
		if len(gs) != 2 {
			t.Fatalf("expected 2 distinct groceries, got %d: %+v", len(gs), gs)
		}
		seen := map[string]bool{}
		for _, g := range gs {
			if seen[g.Name] {
				t.Errorf("duplicate display name %q (ToUpper-fold mismatch)", g.Name)
			}
			seen[g.Name] = true
		}
		if !seen["Σ"] || !seen["ς"] {
			t.Fatalf("display case not preserved: %+v", gs)
		}
	})

	t.Run("Groceries_RemoveNotFound", func(t *testing.T) {
		s := newStore(t, factory)
		err := s.RemoveGrocery(ctx, "absent")
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("Groceries_EmptyName_InvalidArgument", func(t *testing.T) {
		s := newStore(t, factory)
		for name, fn := range map[string]func() error{
			"AddGrocery":    func() error { return s.AddGrocery(ctx, storage.GroceryItem{Name: "  "}) },
			"RemoveGrocery": func() error { return s.RemoveGrocery(ctx, "") },
			"HasGrocery":    func() error { _, err := s.HasGrocery(ctx, " "); return err },
		} {
			if err := fn(); !errors.Is(err, storage.ErrInvalidArgument) {
				t.Errorf("%s: got %v, want ErrInvalidArgument", name, err)
			}
		}
	})

	t.Run("Groceries_EmptyEmbedding_RoundTripsNil", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.AddGrocery(ctx, storage.GroceryItem{Name: "bread"}); err != nil {
			t.Fatalf("AddGrocery: %v", err)
		}
		gs, err := s.ListGroceries(ctx)
		if err != nil || len(gs) != 1 {
			t.Fatalf("ListGroceries: %v len=%d", err, len(gs))
		}
		if gs[0].Embedding != nil {
			t.Fatalf("empty embedding read back as %v, want nil", gs[0].Embedding)
		}
	})

	t.Run("Flyer_RoundTrip_WithStores", func(t *testing.T) {
		s := newStore(t, factory)
		f := storage.Flyer{
			ID:        1,
			ValidFrom: mustTime(t, "2025-01-01T00:00:00.123456789Z"),
			ValidTo:   mustTime(t, "2025-01-31T23:59:59.987654321Z"),
			Name:      "Winter Sale",
			Merchant:  "Superstore",
			Stores: []storage.Store{
				{ID: 1, Address: "123 Main St", City: "Anytown", Province: "ON", PostalCode: "A1A 1A1"},
				{ID: 2, Address: "456 Oak Ave", City: "Otherville", Province: "BC", PostalCode: "V1V 1V1"},
			},
		}
		if err := s.AddFlyer(ctx, f); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		got, err := s.GetFlyer(ctx, 1)
		if err != nil {
			t.Fatalf("GetFlyer: %v", err)
		}
		if !got.ValidFrom.Equal(f.ValidFrom) || !got.ValidTo.Equal(f.ValidTo) {
			t.Fatalf("times not preserved: got %v/%v want %v/%v (sub-second)", got.ValidFrom, got.ValidTo, f.ValidFrom, f.ValidTo)
		}
		if got.Name != f.Name || got.Merchant != f.Merchant {
			t.Fatalf("name/merchant mismatch: %+v", got)
		}
		if !reflect.DeepEqual(got.Stores, f.Stores) {
			t.Fatalf("stores not preserved: got %+v want %+v", got.Stores, f.Stores)
		}
		has, err := s.HasFlyer(ctx, 1)
		if err != nil || !has {
			t.Fatalf("HasFlyer(1) = %v, err=%v", has, err)
		}
	})

	t.Run("Flyer_NilStores_RoundTripsNil", func(t *testing.T) {
		s := newStore(t, factory)
		f := storage.Flyer{ID: 1, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "No Stores", Merchant: "M"}
		if f.Stores != nil {
			t.Fatal("test setup: Stores should be nil")
		}
		if err := s.AddFlyer(ctx, f); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		got, err := s.GetFlyer(ctx, 1)
		if err != nil {
			t.Fatalf("GetFlyer: %v", err)
		}
		if got.Stores != nil {
			t.Fatalf("nil Stores round-tripped to non-nil: %#v", got.Stores)
		}
	})

	t.Run("Flyer_NonNilEmptyStores_Preserved", func(t *testing.T) {
		s := newStore(t, factory)
		empty := []storage.Store{}
		f := storage.Flyer{ID: 1, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "No Stores", Merchant: "M", Stores: empty}
		if err := s.AddFlyer(ctx, f); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		got, err := s.GetFlyer(ctx, 1)
		if err != nil {
			t.Fatalf("GetFlyer: %v", err)
		}
		if got.Stores == nil || len(got.Stores) != 0 {
			t.Fatalf("non-nil empty Stores not preserved: nil=%v", got.Stores == nil)
		}
	})

	t.Run("Flyer_DuplicateID", func(t *testing.T) {
		s := newStore(t, factory)
		f := storage.Flyer{ID: 1, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "A", Merchant: "M"}
		if err := s.AddFlyer(ctx, f); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		err := s.AddFlyer(ctx, storage.Flyer{ID: 1, ValidFrom: mustTime(t, "2025-02-01T00:00:00Z"), Name: "B", Merchant: "M"})
		if !errors.Is(err, storage.ErrDuplicate) {
			t.Fatalf("got %v, want ErrDuplicate", err)
		}
	})

	t.Run("Flyer_GetNotFound", func(t *testing.T) {
		s := newStore(t, factory)
		_, err := s.GetFlyer(ctx, 999)
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("Flyer_ZeroValidTo_NoExpiry", func(t *testing.T) {
		s := newStore(t, factory)
		f := storage.Flyer{ID: 1, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "Forever", Merchant: "M"}
		if err := s.AddFlyer(ctx, f); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		got, err := s.GetFlyer(ctx, 1)
		if err != nil {
			t.Fatalf("GetFlyer: %v", err)
		}
		if !got.ValidTo.IsZero() {
			t.Fatalf("zero ValidTo not preserved: got %v", got.ValidTo)
		}
	})

	t.Run("Flyer_ListMultiple", func(t *testing.T) {
		s := newStore(t, factory)
		for id := int64(1); id <= 3; id++ {
			f := storage.Flyer{ID: id, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: fmt.Sprintf("F%d", id), Merchant: "M"}
			if err := s.AddFlyer(ctx, f); err != nil {
				t.Fatalf("AddFlyer %d: %v", id, err)
			}
		}
		fs, err := s.ListFlyers(ctx)
		if err != nil || len(fs) != 3 {
			t.Fatalf("ListFlyers: err=%v len=%d", err, len(fs))
		}
		seen := map[int64]bool{}
		for _, f := range fs {
			seen[f.ID] = true
		}
		for id := int64(1); id <= 3; id++ {
			if !seen[id] {
				t.Fatalf("flyer %d missing from list", id)
			}
		}
	})

	t.Run("FlyerItem_AddToUnknownFlyer_ErrNotFound", func(t *testing.T) {
		s := newStore(t, factory)
		err := s.AddFlyerItem(ctx, storage.FlyerItem{ID: 1, FlyerID: 999, Name: "orphan"})
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("FlyerItem_RoundTrip", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.AddFlyer(ctx, storage.Flyer{ID: 10, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "F", Merchant: "M"}); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		item := storage.FlyerItem{
			ID: 3, FlyerID: 10, Name: "Milk 2L", Brand: "Dairy", Price: "$4.99",
			ImageURL: "http://example.com/i.jpg", VideoURL: "http://example.com/v.mp4", DisplayType: 2,
		}
		if err := s.AddFlyerItem(ctx, item); err != nil {
			t.Fatalf("AddFlyerItem: %v", err)
		}
		items, err := s.ListFlyerItems(ctx, 10)
		if err != nil || len(items) != 1 {
			t.Fatalf("ListFlyerItems: err=%v len=%d", err, len(items))
		}
		got := items[0]
		if got.ID != item.ID || got.FlyerID != item.FlyerID || got.Name != item.Name || got.Brand != item.Brand ||
			got.Price != item.Price || got.ImageURL != item.ImageURL || got.VideoURL != item.VideoURL || got.DisplayType != item.DisplayType {
			t.Fatalf("item round-trip mismatch: %+v", got)
		}
		has, err := s.HasFlyerItem(ctx, 10, 3)
		if err != nil || !has {
			t.Fatalf("HasFlyerItem(10,3) = %v, err=%v", has, err)
		}
		// Remove it.
		if err := s.RemoveFlyerItem(ctx, 10, 3); err != nil {
			t.Fatalf("RemoveFlyerItem: %v", err)
		}
		items, _ = s.ListFlyerItems(ctx, 10)
		if len(items) != 0 {
			t.Fatalf("expected empty after remove, got %d", len(items))
		}
	})

	t.Run("FlyerItem_Duplicate_SameFlyer", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.AddFlyer(ctx, storage.Flyer{ID: 10, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "F", Merchant: "M"}); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		if err := s.AddFlyerItem(ctx, storage.FlyerItem{ID: 5, FlyerID: 10, Name: "a"}); err != nil {
			t.Fatalf("first AddFlyerItem: %v", err)
		}
		err := s.AddFlyerItem(ctx, storage.FlyerItem{ID: 5, FlyerID: 10, Name: "dup"})
		if !errors.Is(err, storage.ErrDuplicate) {
			t.Fatalf("got %v, want ErrDuplicate", err)
		}
	})

	t.Run("FlyerItem_SameID_DifferentFlyer_Allowed", func(t *testing.T) {
		s := newStore(t, factory)
		for id := int64(1); id <= 2; id++ {
			if err := s.AddFlyer(ctx, storage.Flyer{ID: id, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "F", Merchant: "M"}); err != nil {
				t.Fatalf("AddFlyer %d: %v", id, err)
			}
		}
		// Item IDs are per-flyer: the same ID under two flyers must both succeed.
		if err := s.AddFlyerItem(ctx, storage.FlyerItem{ID: 7, FlyerID: 1, Name: "a"}); err != nil {
			t.Fatalf("flyer1 item 7: %v", err)
		}
		if err := s.AddFlyerItem(ctx, storage.FlyerItem{ID: 7, FlyerID: 2, Name: "b"}); err != nil {
			t.Fatalf("flyer2 item 7 (same ID): %v", err)
		}
	})

	t.Run("FlyerItem_RemoveNotFound", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.AddFlyer(ctx, storage.Flyer{ID: 10, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "F", Merchant: "M"}); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		err := s.RemoveFlyerItem(ctx, 10, 999)
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("Flyer_Remove_CascadesItems", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.AddFlyer(ctx, storage.Flyer{ID: 10, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "F", Merchant: "M"}); err != nil {
			t.Fatalf("AddFlyer: %v", err)
		}
		for i := int64(1); i <= 3; i++ {
			if err := s.AddFlyerItem(ctx, storage.FlyerItem{ID: i, FlyerID: 10, Name: "it"}); err != nil {
				t.Fatalf("AddFlyerItem %d: %v", i, err)
			}
		}
		if err := s.RemoveFlyer(ctx, 10); err != nil {
			t.Fatalf("RemoveFlyer: %v", err)
		}
		if has, _ := s.HasFlyer(ctx, 10); has {
			t.Fatal("flyer still present after remove")
		}
		items, err := s.ListFlyerItems(ctx, 10)
		if err != nil || len(items) != 0 {
			t.Fatalf("items not cascaded: err=%v len=%d", err, len(items))
		}
	})

	t.Run("PruneExpired_RemovesStale_KeepsCurrentAndNoExpiry", func(t *testing.T) {
		s := newStore(t, factory)
		now := mustTime(t, "2025-06-15T12:00:00Z")

		expired := storage.Flyer{ID: 1, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), ValidTo: mustTime(t, "2025-06-01T00:00:00Z"), Name: "Expired", Merchant: "M"}
		current := storage.Flyer{ID: 2, ValidFrom: mustTime(t, "2025-06-01T00:00:00Z"), ValidTo: mustTime(t, "2025-06-30T00:00:00Z"), Name: "Current", Merchant: "M"}
		noExpiry := storage.Flyer{ID: 3, ValidFrom: mustTime(t, "2025-01-01T00:00:00Z"), Name: "Forever", Merchant: "M"}

		for _, f := range []storage.Flyer{expired, current, noExpiry} {
			if err := s.AddFlyer(ctx, f); err != nil {
				t.Fatalf("AddFlyer %d: %v", f.ID, err)
			}
		}
		for _, it := range []storage.FlyerItem{{ID: 1, FlyerID: 1, Name: "expired item"}, {ID: 1, FlyerID: 2, Name: "current item"}} {
			if err := s.AddFlyerItem(ctx, it); err != nil {
				t.Fatalf("AddFlyerItem: %v", err)
			}
		}

		if err := s.PruneExpired(ctx, now); err != nil {
			t.Fatalf("PruneExpired: %v", err)
		}

		if has, _ := s.HasFlyer(ctx, 1); has {
			t.Fatal("expired flyer not removed")
		}
		if has, _ := s.HasFlyer(ctx, 2); !has {
			t.Fatal("current flyer wrongly removed")
		}
		if has, _ := s.HasFlyer(ctx, 3); !has {
			t.Fatal("no-expiry flyer wrongly removed")
		}
		items, _ := s.ListFlyerItems(ctx, 1)
		if len(items) != 0 {
			t.Fatalf("expired flyer items not cascaded: %d", len(items))
		}
		items, _ = s.ListFlyerItems(ctx, 2)
		if len(items) != 1 {
			t.Fatalf("current flyer items removed: %d", len(items))
		}

		// Idempotent.
		if err := s.PruneExpired(ctx, now); err != nil {
			t.Fatalf("second PruneExpired: %v", err)
		}
		if has, _ := s.HasFlyer(ctx, 2); !has {
			t.Fatal("current flyer removed by idempotent prune")
		}
	})

	t.Run("PruneExpired_SubSecond_Precision", func(t *testing.T) {
		s := newStore(t, factory)
		// Same wall-second boundaries: valid_to before a fractional now must prune.
		nowLate := mustTime(t, "2025-03-01T10:00:00.500Z")
		validToEarly := mustTime(t, "2025-03-01T10:00:00.000Z")
		if err := s.AddFlyer(ctx, storage.Flyer{ID: 1, ValidFrom: validToEarly.Add(-time.Hour), ValidTo: validToEarly, Name: "Early", Merchant: "M"}); err != nil {
			t.Fatalf("AddFlyer early: %v", err)
		}
		if err := s.PruneExpired(ctx, nowLate); err != nil {
			t.Fatalf("PruneExpired: %v", err)
		}
		if has, _ := s.HasFlyer(ctx, 1); has {
			t.Fatal("valid_to 10:00:00.000Z should be pruned at now 10:00:00.500Z")
		}

		// valid_to after a fractional now must survive.
		nowEarly := mustTime(t, "2025-03-01T10:00:00.400Z")
		validToLate := mustTime(t, "2025-03-01T10:00:00.600Z")
		if err := s.AddFlyer(ctx, storage.Flyer{ID: 2, ValidFrom: validToLate.Add(-time.Hour), ValidTo: validToLate, Name: "Late", Merchant: "M"}); err != nil {
			t.Fatalf("AddFlyer late: %v", err)
		}
		if err := s.PruneExpired(ctx, nowEarly); err != nil {
			t.Fatalf("PruneExpired: %v", err)
		}
		if has, _ := s.HasFlyer(ctx, 2); !has {
			t.Fatal("valid_to 10:00:00.600Z should survive prune at now 10:00:00.400Z")
		}
	})

	t.Run("PruneExpired_NoFlyers", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.PruneExpired(ctx, time.Now()); err != nil {
			t.Fatalf("PruneExpired on empty store: %v", err)
		}
	})

	t.Run("Close_Idempotent", func(t *testing.T) {
		s := factory(t)
		if err := s.Close(); err != nil {
			t.Fatalf("first Close: %v", err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("second Close: %v", err)
		}
	})

	t.Run("Close_NoPanic_OnFurtherCalls", func(t *testing.T) {
		s := newStore(t, factory)
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		// Must return an error, not panic with a nil-deref.
		_, err := s.ListGroceries(ctx)
		if err == nil {
			t.Fatal("expected error after Close, got nil")
		}
	})

	t.Run("Concurrent_AddDistinctGroceries", func(t *testing.T) {
		s := newStore(t, factory)
		const n = 40
		// Pre-compute the embedding so mustEmbed (which can call t.Fatalf) is
		// never invoked from a non-test goroutine.
		emb := mustEmbed(t, 3)
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(i int) {
				defer wg.Done()
				if err := s.AddGrocery(ctx, storage.GroceryItem{Name: fmt.Sprintf("item_%03d", i), Embedding: emb}); err != nil {
					t.Errorf("AddGrocery item_%03d: %v", i, err)
				}
			}(i)
		}
		wg.Wait()
		gs, err := s.ListGroceries(ctx)
		if err != nil {
			t.Fatalf("ListGroceries: %v", err)
		}
		if len(gs) != n {
			t.Fatalf("expected %d groceries, got %d (torn writes)", n, len(gs))
		}
	})

	t.Run("Concurrent_AddSameGrocery_SingleSuccess", func(t *testing.T) {
		s := newStore(t, factory)
		const n = 20
		// Pre-compute the embedding so mustEmbed (which can call t.Fatalf) is
		// never invoked from a non-test goroutine.
		emb := mustEmbed(t, 3)
		var (
			wg          sync.WaitGroup
			mu          sync.Mutex
			successes   int
			dupErrors   int
			otherErrors int
		)
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				err := s.AddGrocery(ctx, storage.GroceryItem{Name: "milk", Embedding: emb})
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					successes++
				case errors.Is(err, storage.ErrDuplicate):
					dupErrors++
				default:
					otherErrors++
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()
		if successes != 1 {
			t.Errorf("expected exactly 1 success, got %d", successes)
		}
		if dupErrors != n-1 {
			t.Errorf("expected %d ErrDuplicate, got %d", n-1, dupErrors)
		}
		if otherErrors != 0 {
			t.Errorf("expected 0 other errors, got %d", otherErrors)
		}
		gs, err := s.ListGroceries(ctx)
		if err != nil || len(gs) != 1 {
			t.Fatalf("expected 1 stored grocery, got %d (err=%v)", len(gs), err)
		}
	})
}
