package scrape

import (
	"context"
	"fmt"
	"testing"
	"time"

	"stfg/internal/storage"
)

// MockFinder is a mock implementation of flyerfinder.FlyerFinder for testing.
type MockFinder struct {
	FindFlyersFunc     func(postalCode string) ([]storage.Flyer, error)
	FindFlyerItemsFunc func(flyerID int64) ([]storage.FlyerItem, error)
}

// nopEmbedder is a minimal Embedder that returns an empty slice. It is used
// by the existing Reconcile tests that do not exercise the embedding path.
type nopEmbedder struct{}

func (nopEmbedder) CreateGroceryEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{}, nil
}

// fakeEmbedder records every call and returns a deterministic non-empty
// vector when one is requested. tests can later assert on the recorded text
// slice to confirm the reconciler only embedded when expected.
type fakeEmbedder struct {
	calls []string
}

func (f *fakeEmbedder) CreateGroceryEmbedding(ctx context.Context, text string) ([]float32, error) {
	f.calls = append(f.calls, text)
	return []float32{0.1, 0.2, 0.3}, nil
}

// recordingEmbedder is the same idea but lets each test pre-program the
// return value (e.g. nil for an empty-text call) without bypassing the call
// counter.
type recordingEmbedder struct {
	calls   []string
	returns []float32
	err     error
}

func (r *recordingEmbedder) CreateGroceryEmbedding(ctx context.Context, text string) ([]float32, error) {
	r.calls = append(r.calls, text)
	if r.err != nil {
		return nil, r.err
	}
	return r.returns, nil
}

// validFlyer returns a flyer with a validity window that contains `now`,
// matching the expectations of the existing Reconcile tests.
func validFlyer(id int64) storage.Flyer {
	return storage.Flyer{
		ID:        id,
		Name:      "Some Flyer",
		Merchant:  "Test",
		ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC),
	}
}

func (m *MockFinder) FindFlyers(postalCode string) ([]storage.Flyer, error) {
	if m.FindFlyersFunc != nil {
		return m.FindFlyersFunc(postalCode)
	}
	return nil, nil
}

func (m *MockFinder) FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error) {
	if m.FindFlyerItemsFunc != nil {
		return m.FindFlyerItemsFunc(flyerID)
	}
	return nil, nil
}

// mockStore is a minimal in-memory store implementing storage.Storage for tests.
type mockStore struct {
	flyers []storage.Flyer
	items  map[int64][]storage.FlyerItem
}

func (m *mockStore) Close() error { return nil }

func (m *mockStore) Migrate(ctx context.Context) error { return nil }

func (m *mockStore) AddGrocery(ctx context.Context, item storage.GroceryItem) error { return nil }

func (m *mockStore) RemoveGrocery(ctx context.Context, name string) error { return nil }

func (m *mockStore) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) {
	return nil, nil
}

func (m *mockStore) HasGrocery(ctx context.Context, name string) (bool, error) { return false, nil }

func (m *mockStore) AddFlyer(ctx context.Context, flyer storage.Flyer) error {
	m.flyers = append(m.flyers, flyer)
	return nil
}

func (m *mockStore) RemoveFlyer(ctx context.Context, id int64) error {
	var kept []storage.Flyer
	for _, f := range m.flyers {
		if f.ID != id {
			kept = append(kept, f)
		}
	}
	m.flyers = kept
	return nil
}

func (m *mockStore) GetFlyer(ctx context.Context, id int64) (*storage.Flyer, error) {
	for _, f := range m.flyers {
		if f.ID == id {
			return &f, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockStore) ListFlyers(ctx context.Context) ([]storage.Flyer, error) {
	return m.flyers, nil
}

func (m *mockStore) HasFlyer(ctx context.Context, id int64) (bool, error) {
	for _, f := range m.flyers {
		if f.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockStore) AddFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	if m.items == nil {
		m.items = make(map[int64][]storage.FlyerItem)
	}
	m.items[item.FlyerID] = append(m.items[item.FlyerID], item)
	return nil
}

func (m *mockStore) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error {
	items := m.items[flyerID]
	var kept []storage.FlyerItem
	for _, it := range items {
		if it.ID != itemID {
			kept = append(kept, it)
		}
	}
	m.items[flyerID] = kept
	return nil
}

func (m *mockStore) ListFlyerItems(ctx context.Context, flyerID int64) ([]storage.FlyerItem, error) {
	return m.items[flyerID], nil
}

func (m *mockStore) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) {
	for _, it := range m.items[flyerID] {
		if it.ID == itemID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockStore) UpdateFlyerItem(ctx context.Context, item storage.FlyerItem) error {
	items := m.items[item.FlyerID]
	for i := range items {
		if items[i].ID == item.ID {
			items[i] = item
			m.items[item.FlyerID] = items
			return nil
		}
	}
	return storage.ErrNotFound
}

func (m *mockStore) PruneExpired(ctx context.Context, now time.Time) error {
	var kept []storage.Flyer
	for _, f := range m.flyers {
		if now.Before(f.ValidTo) {
			kept = append(kept, f)
		}
	}
	m.flyers = kept
	return nil
}

func now() time.Time {
	return time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
}

// TestReconcile_AcceptsAnyFlyerFinder verifies the reconciler works with any FlyerFinder.
func TestReconcile_AcceptsAnyFlyerFinder(t *testing.T) {
	mock := &MockFinder{
		FindFlyersFunc:     func(s string) ([]storage.Flyer, error) { return nil, nil },
		FindFlyerItemsFunc: func(i int64) ([]storage.FlyerItem, error) { return nil, nil },
	}
	store := &mockStore{}
	opts := ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{},
		Embedder:             nopEmbedder{},
	}
	if err := Reconcile(context.Background(), mock, store, opts); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	flyers, _ := store.ListFlyers(context.Background())
	if len(flyers) != 0 {
		t.Errorf("expected 0 flyers, got %d", len(flyers))
	}
}

// TestFlyerIsValid_ValidFlyer checks a flyer whose window contains now.
func TestFlyerIsValid_ValidFlyer(t *testing.T) {
	f := storage.Flyer{
		ID:        1,
		Name:      "Valid",
		Merchant:  "Test",
		ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	if !FlyerIsValid(f) {
		t.Error("expected flyer with past ValidFrom and future ValidTo to be valid")
	}
}

// TestFlyerIsValid_ExpiredFlyer checks a flyer whose ValidTo is in the past.
func TestFlyerIsValid_ExpiredFlyer(t *testing.T) {
	f := storage.Flyer{
		ID:        2,
		Name:      "Expired",
		Merchant:  "Test",
		ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2020, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	if FlyerIsValid(f) {
		t.Error("expected expired flyer to be invalid")
	}
}

// TestFlyerIsValid_FutureFlyer checks a flyer whose ValidFrom is in the future.
func TestFlyerIsValid_FutureFlyer(t *testing.T) {
	f := storage.Flyer{
		ID:        3,
		Name:      "Future",
		Merchant:  "Test",
		ValidFrom: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	if FlyerIsValid(f) {
		t.Error("expected future flyer to be invalid")
	}
}

// TestReconcile_EmptyWhitelist verifies that no flyers are added with an empty whitelist.
func TestReconcile_EmptyWhitelist(t *testing.T) {
	mock := &MockFinder{
		FindFlyersFunc: func(s string) ([]storage.Flyer, error) {
			return []storage.Flyer{
				{
					ID:        10,
					Name:      "Some Flyer",
					Merchant:  "Test",
					ValidFrom: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					ValidTo:   time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC),
				},
			}, nil
		},
		FindFlyerItemsFunc: func(i int64) ([]storage.FlyerItem, error) { return nil, nil },
	}
	store := &mockStore{}
	opts := ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{},
		Embedder:             nopEmbedder{},
	}
	if err := Reconcile(context.Background(), mock, store, opts); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	flyers, _ := store.ListFlyers(context.Background())
	if len(flyers) != 0 {
		t.Errorf("expected 0 flyers with empty whitelist, got %d", len(flyers))
	}
}

// runReconcile drives a single Reconcile against the given store and
// MockFinder wiring. It does not create the store; tests are responsible for
// carrying the same store across calls when they want to observe what the
// reconciler does on a re-scrape.
func runReconcile(t *testing.T, store storage.StorageProvider, finderItems []storage.FlyerItem, embedder Embedder) {
	t.Helper()
	mock := &MockFinder{
		FindFlyersFunc: func(s string) ([]storage.Flyer, error) {
			return []storage.Flyer{validFlyer(10)}, nil
		},
		FindFlyerItemsFunc: func(id int64) ([]storage.FlyerItem, error) {
			return finderItems, nil
		},
	}
	opts := ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{"Test"},
		Embedder:             embedder,
	}
	if err := Reconcile(context.Background(), mock, store, opts); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
}

func TestReconcile_NewItem_EmbedsAndStores(t *testing.T) {
	embedder := &fakeEmbedder{}
	store := &mockStore{}
	runReconcile(t, store, []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "Milk 2L", Brand: "Dairy"},
	}, embedder)

	if got := embedder.calls; len(got) != 1 || got[0] != "Dairy Milk 2L" {
		t.Fatalf("embedder calls = %v, want [\"Dairy Milk 2L\"]", got)
	}
	stored, _ := store.ListFlyerItems(context.Background(), 10)
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored item, got %d", len(stored))
	}
	if !equalFloat32(stored[0].Embedding, []float32{0.1, 0.2, 0.3}) {
		t.Fatalf("stored embedding = %v, want fake vector", stored[0].Embedding)
	}
}

func TestReconcile_SecondRun_NoEmbed_NoWrite(t *testing.T) {
	embedder := &fakeEmbedder{}
	store := &mockStore{}
	items := []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "Milk 2L", Brand: "Dairy"},
	}
	runReconcile(t, store, items, embedder)
	calls := len(embedder.calls)
	before, _ := store.ListFlyerItems(context.Background(), 10)
	if len(before) != 1 {
		t.Fatalf("expected 1 stored item after first run, got %d", len(before))
	}

	runReconcile(t, store, items, embedder)
	if len(embedder.calls) != calls {
		t.Fatalf("expected zero embed calls on second reconcile, got %d (calls=%v)", len(embedder.calls)-calls, embedder.calls[calls:])
	}
	after, _ := store.ListFlyerItems(context.Background(), 10)
	if !equalFloat32(after[0].Embedding, before[0].Embedding) {
		t.Fatalf("stored embedding changed on no-op reconcile: before=%v after=%v", before[0].Embedding, after[0].Embedding)
	}
}

func TestReconcile_TextChange_Reembed(t *testing.T) {
	embedder := &fakeEmbedder{}
	store := &mockStore{}
	runReconcile(t, store, []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "Milk 2L", Brand: "Dairy"},
	}, embedder)
	calls := len(embedder.calls)

	runReconcile(t, store, []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "Milk 4L", Brand: "Dairy"}, // name change
	}, embedder)
	if len(embedder.calls)-calls != 1 {
		t.Fatalf("expected exactly 1 embed call on name change, got %d", len(embedder.calls)-calls)
	}
	stored, _ := store.ListFlyerItems(context.Background(), 10)
	if stored[0].Name != "Milk 4L" {
		t.Fatalf("stored name not updated: %q", stored[0].Name)
	}

	calls = len(embedder.calls)
	runReconcile(t, store, []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "Milk 4L", Brand: "Dairyland"}, // brand change
	}, embedder)
	if len(embedder.calls)-calls != 1 {
		t.Fatalf("expected exactly 1 embed call on brand change, got %d", len(embedder.calls)-calls)
	}
}

func TestReconcile_EmptyBrandAndName_NotStored(t *testing.T) {
	// Items with empty brand and name are unidentifiable for matching and
	// have no embedding text; drop them before any I/O.
	embedder := &fakeEmbedder{}
	store := &mockStore{}
	runReconcile(t, store, []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "", Brand: ""},
	}, embedder)
	if len(embedder.calls) != 0 {
		t.Fatalf("expected zero embed calls for empty brand+name, got %d", len(embedder.calls))
	}
	stored, _ := store.ListFlyerItems(context.Background(), 10)
	if len(stored) != 0 {
		t.Fatalf("empty-text items must not be stored; got %d items", len(stored))
	}
}

func TestReconcile_ExistingFlyerGainsNewItem_OldHasFlyerSkip(t *testing.T) {
	// Pre-existing flyer (same id) on a re-scrape must still reconcile its
	// items: the old HasFlyer-skip that the plan removes would silently
	// skip this case. Item 1 is pre-populated with an embedding so only the
	// newly scraped item 2 should be embedded.
	embedder := &fakeEmbedder{}
	existingEmbed := []float32{0.9, 0.9, 0.9}
	store := &mockStore{
		flyers: []storage.Flyer{validFlyer(10)},
		items: map[int64][]storage.FlyerItem{
			10: {{ID: 1, FlyerID: 10, Name: "Milk 2L", Brand: "Dairy", Embedding: existingEmbed}},
		},
	}
	mock := &MockFinder{
		FindFlyersFunc: func(s string) ([]storage.Flyer, error) {
			return []storage.Flyer{validFlyer(10)}, nil
		},
		FindFlyerItemsFunc: func(id int64) ([]storage.FlyerItem, error) {
			return []storage.FlyerItem{
				{ID: 1, FlyerID: 10, Name: "Milk 2L", Brand: "Dairy"},
				{ID: 2, FlyerID: 10, Name: "Bread", Brand: "Bakery"}, // new
			}, nil
		},
	}
	if err := Reconcile(context.Background(), mock, store, ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{"Test"},
		Embedder:             embedder,
	}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(embedder.calls) != 1 || embedder.calls[0] != "Bakery Bread" {
		t.Fatalf("expected 1 embed call for the new item only, got %v", embedder.calls)
	}
	items, _ := store.ListFlyerItems(context.Background(), 10)
	if len(items) != 2 {
		t.Fatalf("expected 2 stored items, got %d", len(items))
	}
	for _, it := range items {
		switch it.ID {
		case 1:
			if !equalFloat32(it.Embedding, existingEmbed) {
				t.Fatalf("existing item embedding overwritten: got %v want %v", it.Embedding, existingEmbed)
			}
		case 2:
			if !equalFloat32(it.Embedding, []float32{0.1, 0.2, 0.3}) {
				t.Fatalf("new item embedding = %v, want fake vector", it.Embedding)
			}
		default:
			t.Fatalf("unexpected stored item id %d", it.ID)
		}
	}
}

func TestReconcile_EmbedError_Aborts(t *testing.T) {
	embedder := &recordingEmbedder{err: fmt.Errorf("boom")}
	mock := &MockFinder{
		FindFlyersFunc: func(s string) ([]storage.Flyer, error) {
			return []storage.Flyer{validFlyer(10)}, nil
		},
		FindFlyerItemsFunc: func(id int64) ([]storage.FlyerItem, error) {
			return []storage.FlyerItem{
				{ID: 1, FlyerID: 10, Name: "Milk 2L", Brand: "Dairy"},
			}, nil
		},
	}
	store := &mockStore{}
	err := Reconcile(context.Background(), mock, store, ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{"Test"},
		Embedder:             embedder,
	})
	if err == nil {
		t.Fatal("expected error from embedder failure, got nil")
	}
}

func TestReconcile_NilEmbedder_Errors(t *testing.T) {
	mock := &MockFinder{
		FindFlyersFunc:     func(s string) ([]storage.Flyer, error) { return nil, nil },
		FindFlyerItemsFunc: func(i int64) ([]storage.FlyerItem, error) { return nil, nil },
	}
	err := Reconcile(context.Background(), mock, &mockStore{}, ScrapeReconcilerOptions{
		PostalCode: "12345",
	})
	if err == nil {
		t.Fatal("expected error for nil embedder, got nil")
	}
}

// equalFloat32 reports element-wise equality of two float32 slices. It is a
// local helper for the reconciler tests; production code does not need it.
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
