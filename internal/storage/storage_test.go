package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Helper functions
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", s, err)
	}
	return tm
}

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

// Test sentinel errors
func TestErrNotFound_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("%w: something", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("errors.Is(wrapped, ErrNotFound) should be true")
	}
	if errors.Is(errors.New("unrelated"), ErrNotFound) {
		t.Error("errors.Is(unrelated, ErrNotFound) should be false")
	}
}

func TestErrDuplicate_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("%w: something", ErrDuplicate)
	if !errors.Is(wrapped, ErrDuplicate) {
		t.Error("errors.Is(wrapped, ErrDuplicate) should be true")
	}
	if errors.Is(errors.New("unrelated"), ErrDuplicate) {
		t.Error("errors.Is(unrelated, ErrDuplicate) should be false")
	}
}

func TestErrInvalidArgument_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("%w: something", ErrInvalidArgument)
	if !errors.Is(wrapped, ErrInvalidArgument) {
		t.Error("errors.Is(wrapped, ErrInvalidArgument) should be true")
	}
	if errors.Is(errors.New("unrelated"), ErrInvalidArgument) {
		t.Error("errors.Is(unrelated, ErrInvalidArgument) should be false")
	}
}

func TestSentinelErrorMessages_Stable(t *testing.T) {
	if ErrNotFound.Error() != "storage: not found" {
		t.Errorf("ErrNotFound.Error() = %q, want %q", ErrNotFound.Error(), "storage: not found")
	}
	if ErrDuplicate.Error() != "storage: duplicate" {
		t.Errorf("ErrDuplicate.Error() = %q, want %q", ErrDuplicate.Error(), "storage: duplicate")
	}
	if ErrInvalidArgument.Error() != "storage: invalid argument" {
		t.Errorf("ErrInvalidArgument.Error() = %q, want %q", ErrInvalidArgument.Error(), "storage: invalid argument")
	}
}

// Test JSON round-trips for canonical types
func TestGroceryItem_JSONRoundTrip(t *testing.T) {
	item := GroceryItem{
		Name:      "milk",
		Embedding: mustEmbed(t, 3),
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out GroceryItem
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Name != item.Name {
		t.Errorf("Name: got %q, want %q", out.Name, item.Name)
	}
	if !sliceEqualFloat32(out.Embedding, item.Embedding) {
		t.Errorf("Embedding: got %v, want %v", out.Embedding, item.Embedding)
	}
	// Test omitempty: empty embedding should be omitted
	empty := GroceryItem{Name: "bread"}
	data2, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty failed: %v", err)
	}
	if strings.Contains(string(data2), "embedding") {
		t.Error("empty Embedding should be omitted from JSON")
	}
}

func TestFlyer_JSONRoundTrip(t *testing.T) {
	flyer := Flyer{
		ID:        1,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer Name",
		Merchant:  "Merchant",
		Stores: []Store{
			{ID: 1, Address: "123 Main St", City: "Anytown", Province: "ON", PostalCode: "A1A 1A1"},
		},
	}
	data, err := json.Marshal(flyer)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out Flyer
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.ID != flyer.ID {
		t.Errorf("ID: got %d, want %d", out.ID, flyer.ID)
	}
	if !out.ValidFrom.Equal(flyer.ValidFrom) {
		t.Errorf("ValidFrom: got %v, want %v", out.ValidFrom, flyer.ValidFrom)
	}
	if !out.ValidTo.Equal(flyer.ValidTo) {
		t.Errorf("ValidTo: got %v, want %v", out.ValidTo, flyer.ValidTo)
	}
	if out.Name != flyer.Name {
		t.Errorf("Name: got %q, want %q", out.Name, flyer.Name)
	}
	if out.Merchant != flyer.Merchant {
		t.Errorf("Merchant: got %q, want %q", out.Merchant, flyer.Merchant)
	}
	if len(out.Stores) != len(flyer.Stores) {
		t.Errorf("Stores length: got %d, want %d", len(out.Stores), len(flyer.Stores))
	} else {
		if out.Stores[0].ID != flyer.Stores[0].ID {
			t.Errorf("Store[0].ID: got %d, want %d", out.Stores[0].ID, flyer.Stores[0].ID)
		}
	}
	// Test omitempty: empty stores should be omitted
	flyer2 := Flyer{ID: 2}
	data2, err := json.Marshal(flyer2)
	if err != nil {
		t.Fatalf("marshal flyer2 failed: %v", err)
	}
	if !strings.Contains(string(data2), `"id":2`) {
		t.Error("expected ID field present")
	}
	if strings.Contains(string(data2), `"stores"`) {
		t.Error("empty Stores should be omitted")
	}
}

func TestFlyerItem_JSONRoundTrip(t *testing.T) {
	item := FlyerItem{
		ID:        10,
		FlyerID:   1,
		Name:      "Item Name",
		Brand:     "Brand",
		Price:     "$4.99",
		ImageURL:  "http://example.com/image.jpg",
		VideoURL:  "http://example.com/video.mp4",
		DisplayType: 1,
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out FlyerItem
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.ID != item.ID {
		t.Errorf("ID: got %d, want %d", out.ID, item.ID)
	}
	if out.FlyerID != item.FlyerID {
		t.Errorf("FlyerID: got %d, want %d", out.FlyerID, item.FlyerID)
	}
	if out.Name != item.Name {
		t.Errorf("Name: got %q, want %q", out.Name, item.Name)
	}
	if out.Brand != item.Brand {
		t.Errorf("Brand: got %q, want %q", out.Brand, item.Brand)
	}
	if out.Price != item.Price {
		t.Errorf("Price: got %q, want %q", out.Price, item.Price)
	}
	if out.ImageURL != item.ImageURL {
		t.Errorf("ImageURL: got %q, want %q", out.ImageURL, item.ImageURL)
	}
	if out.VideoURL != item.VideoURL {
		t.Errorf("VideoURL: got %q, want %q", out.VideoURL, item.VideoURL)
	}
	if out.DisplayType != item.DisplayType {
		t.Errorf("DisplayType: got %d, want %d", out.DisplayType, item.DisplayType)
	}
	// Test that VideoURL is a plain string (not pointer) and empty string for zero value
	zero := FlyerItem{}
	data2, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshal zero failed: %v", err)
	}
	var out2 FlyerItem
	if err := json.Unmarshal(data2, &out2); err != nil {
		t.Fatalf("unmarshal zero failed: %v", err)
	}
	if out2.VideoURL != "" {
		t.Errorf("zero VideoURL should be empty string, got %q", out2.VideoURL)
	}
}

func TestStore_JSONRoundTrip(t *testing.T) {
	store := Store{
		ID:         42,
		Address:    "456 Oak Ave",
		City:       "Another City",
		Province:   "BC",
		PostalCode: "V1V 1V1",
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out Store
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.ID != store.ID {
		t.Errorf("ID: got %d, want %d", out.ID, store.ID)
	}
	if out.Address != store.Address {
		t.Errorf("Address: got %q, want %q", out.Address, store.Address)
	}
	if out.City != store.City {
		t.Errorf("City: got %q, want %q", out.City, store.City)
	}
	if out.Province != store.Province {
		t.Errorf("Province: got %q, want %q", out.Province, store.Province)
	}
	if out.PostalCode != store.PostalCode {
		t.Errorf("PostalCode: got %q, want %q", out.PostalCode, store.PostalCode)
	}
}

// Test that mockStorage satisfies the Storage interface (compile-time check)
func TestStorage_InterfaceContract(t *testing.T) {
	var _ Storage = (*mockStorage)(nil)
}

// Test that Reconcile accepts storage.Storage (signature check via source)
func TestReconcile_NewSignatureCompiles(t *testing.T) {
	// Debug: print current directory
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Logf("Current directory: %s", dir)
	data, err := os.ReadFile("../../internal/reconciler/scrape/engine.go")
	if err != nil {
		t.Fatalf("cannot read engine.go: %v", err)
	}
	content := string(data)
	// Look for the function signature of Reconcile
	// We want to see: storageClient storage.Storage
	if !strings.Contains(content, "storageClient storage.Storage") {
		t.Error(`expected to find "storageClient storage.Storage" in Reconcile signature in engine.go`)
	}
	// Also check that the scrapeClient parameter is of type ScrapeClient (defined in same file)
	// We can check for "scrapeClient ScrapeClient"
	if !strings.Contains(content, "scrapeClient ScrapeClient") {
		t.Error(`expected to find "scrapeClient ScrapeClient" in Reconcile signature in engine.go`)
	}
}

// Mock types for testing
type mockStorage struct{}

func (m *mockStorage) Close() error { return nil }
func (m *mockStorage) Migrate(ctx context.Context) error { return nil }
func (m *mockStorage) AddGrocery(ctx context.Context, item GroceryItem) error { return nil }
func (m *mockStorage) RemoveGrocery(ctx context.Context, name string) error { return nil }
func (m *mockStorage) ListGroceries(ctx context.Context) ([]GroceryItem, error) { return nil, nil }
func (m *mockStorage) HasGrocery(ctx context.Context, name string) (bool, error) { return false, nil }
func (m *mockStorage) AddFlyer(ctx context.Context, flyer Flyer) error { return nil }
func (m *mockStorage) RemoveFlyer(ctx context.Context, id int64) error { return nil }
func (m *mockStorage) GetFlyer(ctx context.Context, id int64) (*Flyer, error) { return nil, nil }
func (m *mockStorage) ListFlyers(ctx context.Context) ([]Flyer, error) { return nil, nil }
func (m *mockStorage) HasFlyer(ctx context.Context, id int64) (bool, error) { return false, nil }
func (m *mockStorage) AddFlyerItem(ctx context.Context, item FlyerItem) error { return nil }
func (m *mockStorage) RemoveFlyerItem(ctx context.Context, flyerID, itemID int64) error { return nil }
func (m *mockStorage) ListFlyerItems(ctx context.Context, flyerID int64) ([]FlyerItem, error) { return nil, nil }
func (m *mockStorage) HasFlyerItem(ctx context.Context, flyerID, itemID int64) (bool, error) { return false, nil }
func (m *mockStorage) PruneExpired(ctx context.Context, now time.Time) error { return nil }

// Grep-style tests

func TestReconcile_DoesNotCallRetailGroupIsValid(t *testing.T) {
	// Debug: print current directory
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Logf("Current directory: %s", dir)
	data, err := os.ReadFile("../../internal/reconciler/scrape/engine.go")
	if err != nil {
		t.Fatalf("cannot read engine.go: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "RetailGroupIsValid") {
		t.Error("RetailGroupIsValid should not appear in engine.go")
	}
	if strings.Contains(content, "retailGroupIsValid") {
		t.Error("retailGroupIsValid should not appear in engine.go")
	}
}

func TestScrapeTypes_NoStorageClient(t *testing.T) {
	// Debug: print current directory
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Logf("Current directory: %s", dir)
	data, err := os.ReadFile("../../internal/reconciler/scrape/types.go")
	if err != nil {
		t.Fatalf("cannot read types.go: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "type StorageClient") {
		t.Error("type StorageClient should not appear in scrape/types.go")
	}
}

func TestScrapeTypes_RetailGroupLocationRemoved(t *testing.T) {
	// Debug: print current directory
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Logf("Current directory: %s", dir)
	data, err := os.ReadFile("../../internal/reconciler/scrape/types.go")
	if err != nil {
		t.Fatalf("cannot read types.go: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "type RetailGroupLocation") {
		t.Error("type RetailGroupLocation should not be present in scrape/types.go")
	}
	// Also check that the flipp client still references it (we can't check without reading flipp/client.go)
	// For now, just check the types.go file.
}

func TestCanonicalTypes_NoDuplicates(t *testing.T) {
	// Debug: print current directory
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Logf("Current directory: %s", dir)
	// We'll check for the canonical types in the storage package and ensure they are defined only once.
	// We'll search for the struct definitions in the storage directory.
	// We'll use a simple approach: read the storage.go and types.go files and count occurrences.
	// But note: the plan says to grep for "type Flyer struct", etc.
	// We'll do that for the storage package only (internal/storage).
	files := []string{
		"./storage.go",
		"./types.go",
	}
	flyerCount := 0
	flyerItemCount := 0
	groceryItemCount := 0
	storeCount := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("cannot read %s: %v", file, err)
		}
		content := string(data)
		flyerCount += strings.Count(content, "type Flyer struct")
		flyerItemCount += strings.Count(content, "type FlyerItem struct")
		groceryItemCount += strings.Count(content, "type GroceryItem struct")
		storeCount += strings.Count(content, "type Store struct")
	}
	if flyerCount != 1 {
		t.Errorf("expected exactly one 'type Flyer struct' in storage package, got %d", flyerCount)
	}
	if flyerItemCount != 1 {
		t.Errorf("expected exactly one 'type FlyerItem struct' in storage package, got %d", flyerItemCount)
	}
	if groceryItemCount != 1 {
		t.Errorf("expected exactly one 'type GroceryItem struct' in storage package, got %d", groceryItemCount)
	}
	if storeCount != 1 {
		t.Errorf("expected exactly one 'type Store struct' in storage package, got %d", storeCount)
	}
}

// Helper function to compare slices of float32
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