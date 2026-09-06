package json

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"stfg/internal/storage"
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

// newTestStorage creates a FileStorage backed by a temporary directory.
func newTestStorage(t *testing.T) (*FileStorage, context.Context) {
	t.Helper()
	tmpDir := t.TempDir()
	ctx := context.Background()
	fs, err := NewJSONAt(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}
	return fs, ctx
}

func TestNewJSON_CreatesCacheDirAndFiles(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Test that NewJSON creates the directory and files
	fs, err := NewJSONAt(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewJSONAt failed: %v", err)
	}
	defer fs.Close()

	// Check directory exists with correct permissions
	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("cache dir does not exist: %v", err)
	}
	mode := dirInfo.Mode().Perm()
	if mode&0o700 != 0o700 {
		t.Errorf("cache dir mode = %o, owner should have rwx permissions (0o700)", mode)
	}

	// Check groceries.json exists and contains empty array
	groceriesPath := filepath.Join(tmpDir, groceriesFile)
	groceriesData, err := os.ReadFile(groceriesPath)
	if err != nil {
		t.Fatalf("groceries.json does not exist: %v", err)
	}
	if string(groceriesData) != "[]\n" {
		t.Errorf("groceries.json content = %q, want empty array", string(groceriesData))
	}

	// Check flyers_index.json exists and contains empty array
	flyersPath := filepath.Join(tmpDir, flyersIndex)
	flyersData, err := os.ReadFile(flyersPath)
	if err != nil {
		t.Fatalf("flyers_index.json does not exist: %v", err)
	}
	if string(flyersData) != "[]\n" {
		t.Errorf("flyers_index.json content = %q, want empty array", string(flyersData))
	}

	// Verify we can use the storage
	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != 0 {
		t.Errorf("expected empty groceries list, got %d", len(groceries))
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add some data to make sure migrate doesn't corrupt existing data
	grocery := storage.GroceryItem{Name: "test", Embedding: mustEmbed(t, 3)}
	if err := fs.AddGrocery(ctx, grocery); err != nil {
		t.Fatalf("failed to add grocery: %v", err)
	}

	flyer := storage.Flyer{
		ID:        1,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Test Flyer",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("failed to add flyer: %v", err)
	}

	// Run migrate multiple times
	for i := 0; i < 3; i++ {
		if err := fs.Migrate(ctx); err != nil {
			t.Fatalf("Migrate() call %d failed: %v", i+1, err)
		}
	}

	// Verify data is still there
	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("failed to list groceries: %v", err)
	}
	if len(groceries) != 1 || groceries[0].Name != "test" {
		t.Errorf("groceries changed after migrate: got %v", groceries)
	}

	flyers, err := fs.ListFlyers(ctx)
	if err != nil {
		t.Fatalf("failed to list flyers: %v", err)
	}
	if len(flyers) != 1 || flyers[0].ID != 1 {
		t.Errorf("flyers changed after migrate: got %v", flyers)
	}
}

func TestAddGrocery_RoundTrip(t *testing.T) {
	fs, ctx := newTestStorage(t)

	item := storage.GroceryItem{
		Name:      "milk",
		Embedding: mustEmbed(t, 5),
	}

	if err := fs.AddGrocery(ctx, item); err != nil {
		t.Fatalf("AddGrocery failed: %v", err)
	}

	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != 1 {
		t.Errorf("expected 1 grocery, got %d", len(groceries))
		return
	}
	if groceries[0].Name != item.Name {
		t.Errorf("grocery name = %q, want %q", groceries[0].Name, item.Name)
	}
	if !sliceEqualFloat32(groceries[0].Embedding, item.Embedding) {
		t.Errorf("grocery embedding mismatch")
	}
}

func TestAddGrocery_DuplicateReturnsErrDuplicate(t *testing.T) {
	fs, ctx := newTestStorage(t)

	item1 := storage.GroceryItem{Name: "Milk", Embedding: mustEmbed(t, 3)}
	item2 := storage.GroceryItem{Name: "milk", Embedding: mustEmbed(t, 3)} // same name, different case

	if err := fs.AddGrocery(ctx, item1); err != nil {
		t.Fatalf("first AddGrocery failed: %v", err)
	}

	err := fs.AddGrocery(ctx, item2)
	if err == nil {
		t.Error("expected ErrDuplicate for duplicate name (case-insensitive)")
		return
	}
	if !errors.Is(err, storage.ErrDuplicate) {
		t.Errorf("error = %v, want storage.ErrDuplicate", err)
	}
	if !strings.Contains(err.Error(), "milk") {
		t.Errorf("error message does not contain item name: %v", err)
	}
}

func TestRemoveGrocery_RoundTrip(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add two items
	item1 := storage.GroceryItem{Name: "apple", Embedding: mustEmbed(t, 2)}
	item2 := storage.GroceryItem{Name: "banana", Embedding: mustEmbed(t, 2)}
	if err := fs.AddGrocery(ctx, item1); err != nil {
		t.Fatalf("AddGrocery apple failed: %v", err)
	}
	if err := fs.AddGrocery(ctx, item2); err != nil {
		t.Fatalf("AddGrocery banana failed: %v", err)
	}

	// Remove one
	if err := fs.RemoveGrocery(ctx, "apple"); err != nil {
		t.Fatalf("RemoveGrocery apple failed: %v", err)
	}

	// List remaining
	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != 1 {
		t.Errorf("expected 1 grocery after removal, got %d", len(groceries))
		return
	}
	if groceries[0].Name != "banana" {
		t.Errorf("remaining grocery = %q, want banana", groceries[0].Name)
	}
}

func TestRemoveGrocery_CaseInsensitive(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add item with mixed case
	item := storage.GroceryItem{Name: "Milk", Embedding: mustEmbed(t, 3)}
	if err := fs.AddGrocery(ctx, item); err != nil {
		t.Fatalf("AddGrocery failed: %v", err)
	}

	// Remove with different case
	if err := fs.RemoveGrocery(ctx, "milk"); err != nil {
		t.Fatalf("RemoveGrocery failed: %v", err)
	}

	// Verify removed
	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != 0 {
		t.Errorf("expected 0 groceries after removal, got %d", len(groceries))
	}
}

func TestRemoveGrocery_NotFound(t *testing.T) {
	fs, ctx := newTestStorage(t)

	err := fs.RemoveGrocery(ctx, "nonexistent")
	if err == nil {
		t.Error("expected ErrNotFound for removing nonexistent item")
		return
	}
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("error = %v, want storage.ErrNotFound", err)
	}
}

func TestListGroceries_Empty(t *testing.T) {
	fs, ctx := newTestStorage(t)

	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if groceries == nil {
		t.Error("ListGroceries returned nil slice, want empty slice")
	}
	if len(groceries) != 0 {
		t.Errorf("expected empty groceries list, got %d items", len(groceries))
	}
}

func TestListGroceries_StableOrder(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add items in specific order
	items := []storage.GroceryItem{
		{Name: "zebra", Embedding: mustEmbed(t, 2)},
		{Name: "apple", Embedding: mustEmbed(t, 2)},
		{Name: "banana", Embedding: mustEmbed(t, 2)},
	}
	for _, item := range items {
		if err := fs.AddGrocery(ctx, item); err != nil {
			t.Fatalf("AddGrocery %s failed: %v", item.Name, err)
		}
	}

	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != 3 {
		t.Errorf("expected 3 groceries, got %d", len(groceries))
		return
	}
	// Check order is preserved (insertion order)
	if groceries[0].Name != "zebra" {
		t.Errorf("first grocery = %q, want zebra", groceries[0].Name)
	}
	if groceries[1].Name != "apple" {
		t.Errorf("second grocery = %q, want apple", groceries[1].Name)
	}
	if groceries[2].Name != "banana" {
		t.Errorf("third grocery = %q, want banana", groceries[2].Name)
	}
}

func TestHasGrocery_PresentAndAbsent(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Test absent item
	has, err := fs.HasGrocery(ctx, "absent")
	if err != nil {
		t.Fatalf("HasGrocery failed: %v", err)
	}
	if has {
		t.Error("HasGrocery for absent item should return false")
	}

	// Add an item
	item := storage.GroceryItem{Name: "present", Embedding: mustEmbed(t, 3)}
	if err := fs.AddGrocery(ctx, item); err != nil {
		t.Fatalf("AddGrocery failed: %v", err)
	}

	// Test present item (exact case)
	has, err = fs.HasGrocery(ctx, "present")
	if err != nil {
		t.Fatalf("HasGrocery failed: %v", err)
	}
	if !has {
		t.Error("HasGrocery for present item should return true")
	}

	// Test present item (different case)
	has, err = fs.HasGrocery(ctx, "PRESENT")
	if err != nil {
		t.Fatalf("HasGrocery failed: %v", err)
	}
	if !has {
		t.Error("HasGrocery for present item (case-insensitive) should return true")
	}
}

func TestAddFlyer_RoundTrip(t *testing.T) {
	fs, ctx := newTestStorage(t)

	flyer := storage.Flyer{
		ID:        42,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Test Flyer",
		Merchant:  "Test Merchant",
		Stores: []storage.Store{
			{ID: 1, Address: "123 Main St", City: "Anytown", Province: "ON", PostalCode: "A1A 1A1"},
		},
	}

	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	flyers, err := fs.ListFlyers(ctx)
	if err != nil {
		t.Fatalf("ListFlyers failed: %v", err)
	}
	if len(flyers) != 1 {
		t.Errorf("expected 1 flyer, got %d", len(flyers))
		return
	}
	if flyers[0].ID != flyer.ID {
		t.Errorf("flyer ID = %d, want %d", flyers[0].ID, flyer.ID)
	}
	if !flyers[0].ValidFrom.Equal(flyer.ValidFrom) {
		t.Errorf("ValidFrom mismatch")
	}
	if !flyers[0].ValidTo.Equal(flyer.ValidTo) {
		t.Errorf("ValidTo mismatch")
	}
	if flyers[0].Name != flyer.Name {
		t.Errorf("flyer name = %q, want %q", flyers[0].Name, flyer.Name)
	}
	if flyers[0].Merchant != flyer.Merchant {
		t.Errorf("flyer merchant = %q, want %q", flyers[0].Merchant, flyer.Merchant)
	}
	if len(flyers[0].Stores) != len(flyer.Stores) {
		t.Errorf("stores length mismatch")
	} else {
		if flyers[0].Stores[0].ID != flyer.Stores[0].ID {
			t.Errorf("store ID mismatch")
		}
	}
}

func TestAddFlyer_DuplicateIDReturnsErrDuplicate(t *testing.T) {
	fs, ctx := newTestStorage(t)

	flyer1 := storage.Flyer{
		ID:        1,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer 1",
		Merchant:  "Merchant 1",
	}
	flyer2 := storage.Flyer{
		ID:        1, // same ID
		ValidFrom: mustTime(t, "2024-02-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-02-29T23:59:59Z"),
		Name:      "Flyer 2",
		Merchant:  "Merchant 2",
	}

	if err := fs.AddFlyer(ctx, flyer1); err != nil {
		t.Fatalf("first AddFlyer failed: %v", err)
	}

	err := fs.AddFlyer(ctx, flyer2)
	if err == nil {
		t.Error("expected ErrDuplicate for duplicate flyer ID")
		return
	}
	if !errors.Is(err, storage.ErrDuplicate) {
		t.Errorf("error = %v, want storage.ErrDuplicate", err)
	}
}

func TestGetFlyer_Found(t *testing.T) {
	fs, ctx := newTestStorage(t)

	flyer := storage.Flyer{
		ID:        99,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Found Flyer",
		Merchant:  "Found Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	result, err := fs.GetFlyer(ctx, flyer.ID)
	if err != nil {
		t.Fatalf("GetFlyer failed: %v", err)
	}
	if result == nil {
		t.Error("GetFlyer returned nil for existing flyer")
		return
	}
	if result.ID != flyer.ID {
		t.Errorf("GetFlyer ID = %d, want %d", result.ID, flyer.ID)
	}
	if !result.ValidFrom.Equal(flyer.ValidFrom) {
		t.Errorf("GetFlyer ValidFrom mismatch")
	}
	if !result.ValidTo.Equal(flyer.ValidTo) {
		t.Errorf("GetFlyer ValidTo mismatch")
	}
	if result.Name != flyer.Name {
		t.Errorf("GetFlyer name = %q, want %q", result.Name, flyer.Name)
	}
	if result.Merchant != flyer.Merchant {
		t.Errorf("GetFlyer merchant = %q, want %q", result.Merchant, flyer.Merchant)
	}
}

func TestGetFlyer_NotFound(t *testing.T) {
	fs, ctx := newTestStorage(t)

	_, err := fs.GetFlyer(ctx, 999) // non-existent ID
	if err == nil {
		t.Error("GetFlyer for non-existent ID should return error")
		return
	}
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("error = %v, want storage.ErrNotFound", err)
	}
}

func TestListFlyers_Empty(t *testing.T) {
	fs, ctx := newTestStorage(t)

	flyers, err := fs.ListFlyers(ctx)
	if err != nil {
		t.Fatalf("ListFlyers failed: %v", err)
	}
	if flyers == nil {
		t.Error("ListFlyers returned nil slice, want empty slice")
	}
	if len(flyers) != 0 {
		t.Errorf("expected empty flyers list, got %d items", len(flyers))
	}
}

func TestListFlyers_Multiple(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add two flyers
	flyer1 := storage.Flyer{
		ID:        1,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "First Flyer",
		Merchant:  "First Merchant",
	}
	flyer2 := storage.Flyer{
		ID:        2,
		ValidFrom: mustTime(t, "2024-02-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-02-29T23:59:59Z"),
		Name:      "Second Flyer",
		Merchant:  "Second Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer1); err != nil {
		t.Fatalf("AddFlyer first failed: %v", err)
	}
	if err := fs.AddFlyer(ctx, flyer2); err != nil {
		t.Fatalf("AddFlyer second failed: %v", err)
	}

	flyers, err := fs.ListFlyers(ctx)
	if err != nil {
		t.Fatalf("ListFlyers failed: %v", err)
	}
	if len(flyers) != 2 {
		t.Errorf("expected 2 flyers, got %d", len(flyers))
		return
	}
	// Check stable order (insertion order)
	if flyers[0].ID != 1 {
		t.Errorf("first flyer ID = %d, want 1", flyers[0].ID)
	}
	if flyers[1].ID != 2 {
		t.Errorf("second flyer ID = %d, want 2", flyers[1].ID)
	}
}

func TestHasFlyer_PresentAndAbsent(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Test absent flyer
	has, err := fs.HasFlyer(ctx, 999)
	if err != nil {
		t.Fatalf("HasFlyer failed: %v", err)
	}
	if has {
		t.Error("HasFlyer for absent flyer should return false")
	}

	// Add a flyer
	flyer := storage.Flyer{
		ID:        123,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Test Flyer",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	// Test present flyer
	has, err = fs.HasFlyer(ctx, flyer.ID)
	if err != nil {
		t.Fatalf("HasFlyer failed: %v", err)
	}
	if !has {
		t.Error("HasFlyer for present flyer should return true")
	}
}

func TestRemoveFlyer_CascadesItems(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add a flyer with items
	flyer := storage.Flyer{
		ID:        100,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer With Items",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	// Add items to the flyer
	item1 := storage.FlyerItem{
		ID:       1,
		FlyerID:  100,
		Name:     "Item 1",
		Brand:    "Brand A",
		Price:    "$1.00",
	}
	item2 := storage.FlyerItem{
		ID:       2,
		FlyerID:  100,
		Name:     "Item 2",
		Brand:    "Brand B",
		Price:    "$2.00",
	}
	if err := fs.AddFlyerItem(ctx, item1); err != nil {
		t.Fatalf("AddFlyerItem 1 failed: %v", err)
	}
	if err := fs.AddFlyerItem(ctx, item2); err != nil {
		t.Fatalf("AddFlyerItem 2 failed: %v", err)
	}

	// Verify items exist
	items, err := fs.ListFlyerItems(ctx, flyer.ID)
	if err != nil {
		t.Fatalf("ListFlyerItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 flyer items, got %d", len(items))
	}

	// Remove the flyer
	if err := fs.RemoveFlyer(ctx, flyer.ID); err != nil {
		t.Fatalf("RemoveFlyer failed: %v", err)
	}

	// Verify flyer is gone
	has, err := fs.HasFlyer(ctx, flyer.ID)
	if err != nil {
		t.Fatalf("HasFlyer failed: %v", err)
	}
	if has {
		t.Error("Flyer should be removed")
	}

	// Verify items are gone (cascade)
	items, err = fs.ListFlyerItems(ctx, flyer.ID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("ListFlyerItems failed unexpectedly: %v", err)
		}
		// ErrNotFound is expected because the items file was removed
	} else {
		if len(items) != 0 {
			t.Errorf("expected 0 flyer items after flyer removal, got %d", len(items))
		}
	}
}

func TestAddFlyerItem_RoundTrip(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// First add a flyer so we have a valid FlyerID
	flyer := storage.Flyer{
		ID:        200,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer for Items",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	item := storage.FlyerItem{
		ID:          10,
		FlyerID:     200,
		Name:        "Test Item",
		Brand:       "Test Brand",
		Price:       "$3.99",
		ImageURL:    "http://example.com/image.jpg",
		VideoURL:    "http://example.com/video.mp4",
		DisplayType: 1,
	}

	if err := fs.AddFlyerItem(ctx, item); err != nil {
		t.Fatalf("AddFlyerItem failed: %v", err)
	}

	items, err := fs.ListFlyerItems(ctx, item.FlyerID)
	if err != nil {
		t.Fatalf("ListFlyerItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 flyer item, got %d", len(items))
		return
	}
	if items[0].ID != item.ID {
		t.Errorf("item ID = %d, want %d", items[0].ID, item.ID)
	}
	if items[0].FlyerID != item.FlyerID {
		t.Errorf("item FlyerID = %d, want %d", items[0].FlyerID, item.FlyerID)
	}
	if items[0].Name != item.Name {
		t.Errorf("item name = %q, want %q", items[0].Name, item.Name)
	}
	if items[0].Brand != item.Brand {
		t.Errorf("item brand = %q, want %q", items[0].Brand, item.Brand)
	}
	if items[0].Price != item.Price {
		t.Errorf("item price = %q, want %q", items[0].Price, item.Price)
	}
	if items[0].ImageURL != item.ImageURL {
		t.Errorf("item imageURL = %q, want %q", items[0].ImageURL, item.ImageURL)
	}
	if items[0].VideoURL != item.VideoURL {
		t.Errorf("item videoURL = %q, want %q", items[0].VideoURL, item.VideoURL)
	}
	if items[0].DisplayType != item.DisplayType {
		t.Errorf("item displayType = %d, want %d", items[0].DisplayType, item.DisplayType)
	}
}

func TestAddFlyerItem_DuplicateIDReturnsErrDuplicate(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add a flyer
	flyer := storage.Flyer{
		ID:        300,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer for Duplicate Test",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	item1 := storage.FlyerItem{
		ID:       5,
		FlyerID:  300,
		Name:     "Item 1",
	}
	item2 := storage.FlyerItem{
		ID:       5, // same ID and FlyerID
		FlyerID:  300,
		Name:     "Item 2",
	}

	if err := fs.AddFlyerItem(ctx, item1); err != nil {
		t.Fatalf("first AddFlyerItem failed: %v", err)
	}

	err := fs.AddFlyerItem(ctx, item2)
	if err == nil {
		t.Error("expected ErrDuplicate for duplicate flyer item ID")
		return
	}
	if !errors.Is(err, storage.ErrDuplicate) {
		t.Errorf("error = %v, want storage.ErrDuplicate", err)
	}
}

func TestRemoveFlyerItem_RoundTrip(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add a flyer
	flyer := storage.Flyer{
		ID:        400,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer for Item Removal",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	// Add two items
	item1 := storage.FlyerItem{
		ID:       10,
		FlyerID:  400,
		Name:     "First Item",
	}
	item2 := storage.FlyerItem{
		ID:       11,
		FlyerID:  400,
		Name:     "Second Item",
	}
	if err := fs.AddFlyerItem(ctx, item1); err != nil {
		t.Fatalf("AddFlyerItem first failed: %v", err)
	}
	if err := fs.AddFlyerItem(ctx, item2); err != nil {
		t.Fatalf("AddFlyerItem second failed: %v", err)
	}

	// Remove one item
	if err := fs.RemoveFlyerItem(ctx, flyer.ID, item1.ID); err != nil {
		t.Fatalf("RemoveFlyerItem failed: %v", err)
	}

	// List remaining items
	items, err := fs.ListFlyerItems(ctx, flyer.ID)
	if err != nil {
		t.Fatalf("ListFlyerItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 flyer item after removal, got %d", len(items))
		return
	}
	if items[0].ID != item2.ID {
		t.Errorf("remaining item ID = %d, want %d", items[0].ID, item2.ID)
	}
	if items[0].Name != item2.Name {
		t.Errorf("remaining item name = %q, want %q", items[0].Name, item2.Name)
	}
}

func TestRemoveFlyerItem_NotFound(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add a flyer
	flyer := storage.Flyer{
		ID:        500,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer for NotFound Test",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	// Try to remove non-existent item
	err := fs.RemoveFlyerItem(ctx, flyer.ID, 999)
	if err == nil {
		t.Error("expected ErrNotFound for removing non-existent flyer item")
		return
	}
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("error = %v, want storage.ErrNotFound", err)
	}
}

func TestListFlyerItems_Empty(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add a flyer with no items
	flyer := storage.Flyer{
		ID:        600,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer With No Items",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	items, err := fs.ListFlyerItems(ctx, flyer.ID)
	if err != nil {
		t.Fatalf("ListFlyerItems failed: %v", err)
	}
	if items == nil {
		t.Error("ListFlyerItems returned nil slice, want empty slice")
	}
	if len(items) != 0 {
		t.Errorf("expected empty flyer items list, got %d items", len(items))
	}
}

