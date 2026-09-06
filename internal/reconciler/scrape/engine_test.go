package scrape

import (
	"context"
	"testing"
	"time"

	"stfg/internal/storage"
)

// MockFinder is a mock implementation of flyerfinder.FlyerFinder for testing.
type MockFinder struct {
	FindFlyersFunc    func(postalCode string) ([]storage.Flyer, error)
	FindFlyerItemsFunc func(flyerID int64) ([]storage.FlyerItem, error)
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

func (m *mockStore) ListGroceries(ctx context.Context) ([]storage.GroceryItem, error) { return nil, nil }

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
		FindFlyersFunc:    func(s string) ([]storage.Flyer, error) { return nil, nil },
		FindFlyerItemsFunc: func(i int64) ([]storage.FlyerItem, error) { return nil, nil },
	}
	store := &mockStore{}
	opts := ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{},
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
	}
	if err := Reconcile(context.Background(), mock, store, opts); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	flyers, _ := store.ListFlyers(context.Background())
	if len(flyers) != 0 {
		t.Errorf("expected 0 flyers with empty whitelist, got %d", len(flyers))
	}
}