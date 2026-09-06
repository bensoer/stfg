package scrape

import (
	"context"
	"os"
	"testing"

	"stfg/internal/storage"
)

// MockFinder is a mock implementation of FlyerFinder for testing.
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

// Helper to set up a temporary cache directory for JSONFileStorage.
func setupTempCacheDir(t *testing.T) string {
	tmpDir := t.TempDir()
	// We need to set HOME so that os.UserCacheDir() returns a path under tmpDir.
	// os.UserCacheDir() on Linux returns $HOME/.cache.
	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })
	return tmpDir
}

// TestReconcile_AcceptsAnyFlyerFinder uses a mock finder to verify the reconciler works with any FlyerFinder.
func TestReconcile_AcceptsAnyFlyerFinder(t *testing.T) {
	// Set up temporary cache for storage.
	_ = setupTempCacheDir(t) // cleanup is automatic via t.Cleanup

	// Arrange: mock finder that returns no flyers.
	mockFinder := &MockFinder{
		FindFlyersFunc:    func(postalCode string) ([]storage.Flyer, error) { return []storage.Flyer{}, nil },
		FindFlyerItemsFunc: func(flyerID int64) ([]storage.FlyerItem, error) { return []storage.FlyerItem{}, nil },
	}
	// Create real storage with temporary cache.
	store, err := storage.NewJSONFileStorage()
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	options := ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{"Test Merchant"}, // whitelist something, but we have no flyers so it won't matter.
	}

	// Act
	err = Reconcile(context.Background(), mockFinder, store, options)
	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	// Assert: no panics, and store still empty.
	flyers, err := store.GetAllRetailGroups()
	if err != nil {
		t.Fatalf("failed to get flyers from store: %v", err)
	}
	if len(flyers) != 0 {
		t.Errorf("expected no flyers in store after reconcile, got %d", len(flyers))
	}
}

// TestFlyerIsValid_HandlesStringDates tests that FlyerIsValid correctly evaluates date strings.
func TestFlyerIsValid_HandlesStringDates(t *testing.T) {
	// We'll use dates far in the past/future to avoid flakiness due to test timing.
	// Case 1: flyer is valid (now between ValidFrom and ValidTo)
	// We'll set ValidTo to a far future date, ValidFrom to a far past date.
	validFrom := "2000-01-02T00:00:00Z"
	validTo := "2030-01-02T00:00:00Z"
	flyerValid := storage.Flyer{
		ID:        1,
		Name:      "Valid Flyer",
		Merchant:  "Test Merchant",
		ValidFrom: validFrom,
		ValidTo:   validTo,
	}
	valid, err := FlyerIsValid(flyerValid)
	if err != nil {
		t.Fatalf("FlyerIsValid returned unexpected error: %v", err)
	}
	if !valid {
		t.Error("expected flyer with past ValidFrom and future ValidTo to be valid")
	}

	// Case 2: flyer is expired (ValidTo in the past)
	flyerExpired := storage.Flyer{
		ID:        2,
		Name:      "Expired Flyer",
		Merchant:  "Test Merchant",
		ValidFrom: "2000-01-02T00:00:00Z",
		ValidTo:   "2001-01-02T00:00:00Z", // expired
	}
	valid, err = FlyerIsValid(flyerExpired)
	if err != nil {
		t.Fatalf("FlyerIsValid returned unexpected error: %v", err)
	}
	if valid {
		t.Error("expected expired flyer to be invalid")
	}

	// Case 3: flyer is not yet valid (ValidFrom in the future)
	flyerFuture := storage.Flyer{
		ID:        3,
		Name:      "Future Flyer",
		Merchant:  "Test Merchant",
		ValidFrom: "2030-01-02T00:00:00Z", // future
		ValidTo:   "2031-01-02T00:00:00Z",
	}
	valid, err = FlyerIsValid(flyerFuture)
	if err != nil {
		t.Fatalf("FlyerIsValid returned unexpected error: %v", err)
	}
	if valid {
		t.Error("expected flyer with future ValidFrom to be invalid")
	}
}

// TestReconcile_EmptyWhitelist tests that when the whitelist is empty, no new flyers are added.
func TestReconcile_EmptyWhitelist(t *testing.T) {
	// Set up temporary cache for storage.
	_ = setupTempCacheDir(t) // cleanup is automatic via t.Cleanup

	// Arrange: mock finder that returns one flyer.
	mockFinder := &MockFinder{
		FindFlyersFunc: func(postalCode string) ([]storage.Flyer, error) {
			return []storage.Flyer{
				{
					ID:        10,
					Name:      "Whitelisted Flyer",
					Merchant:  "Test Merchant",
					ValidFrom: "2000-01-02T00:00:00Z",
					ValidTo:   "2030-01-02T00:00:00Z",
					Stores:    []storage.Store{}, // empty stores for simplicity
				},
			}, nil
		},
		FindFlyerItemsFunc: func(flyerID int64) ([]storage.FlyerItem, error) {
			return []storage.FlyerItem{}, nil
		},
	}
	// Create real storage with temporary cache.
	store, err := storage.NewJSONFileStorage()
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	options := ScrapeReconcilerOptions{
		PostalCode:           "12345",
		RetailGroupWhiteList: []string{}, // empty whitelist
	}

	// Act
	err = Reconcile(context.Background(), mockFinder, store, options)
	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	// Assert: no flyers added because whitelist empty.
	flyers, err := store.GetAllRetailGroups()
	if err != nil {
		t.Fatalf("failed to get flyers from store: %v", err)
	}
	if len(flyers) != 0 {
		t.Errorf("expected no flyers added when whitelist is empty, got %d", len(flyers))
	}
}

// TestFlyerIsValid_InvalidDateString tests that malformed date strings return an error.
func TestFlyerIsValid_InvalidDateString(t *testing.T) {
	flyer := storage.Flyer{
		ID:        4,
		Name:      "Invalid Date Flyer",
		Merchant:  "Test Merchant",
		ValidFrom: "not-a-date",
		ValidTo:   "2020-01-02T00:00:00Z",
	}
	_, err := FlyerIsValid(flyer)
	if err == nil {
		t.Error("expected FlyerIsValid to return an error for invalid date string")
	}
}