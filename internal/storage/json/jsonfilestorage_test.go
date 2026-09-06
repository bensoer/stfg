package json

import (
	"bytes"
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
	"stfg/internal/storage/parity"
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

func TestAddGrocery_EmptyName_ReturnsErrInvalidArgument(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// AddGrocery must reject empty and whitespace-only names.
	for _, name := range []string{"", "   ", "\t\n", "\t"} {
		err := fs.AddGrocery(ctx, storage.GroceryItem{Name: name})
		if !errors.Is(err, storage.ErrInvalidArgument) {
			t.Errorf("AddGrocery(%q): error = %v, want storage.ErrInvalidArgument", name, err)
		}
	}

	// RemoveGrocery must reject empty and whitespace-only names.
	for _, name := range []string{"", "   ", "\t\n", "\t"} {
		err := fs.RemoveGrocery(ctx, name)
		if !errors.Is(err, storage.ErrInvalidArgument) {
			t.Errorf("RemoveGrocery(%q): error = %v, want storage.ErrInvalidArgument", name, err)
		}
	}

	// HasGrocery must reject empty and whitespace-only names.
	for _, name := range []string{"", "   ", "\t\n", "\t"} {
		_, err := fs.HasGrocery(ctx, name)
		if !errors.Is(err, storage.ErrInvalidArgument) {
			t.Errorf("HasGrocery(%q): error = %v, want storage.ErrInvalidArgument", name, err)
		}
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
		ID:      1,
		FlyerID: 100,
		Name:    "Item 1",
		Brand:   "Brand A",
		Price:   "$1.00",
	}
	item2 := storage.FlyerItem{
		ID:      2,
		FlyerID: 100,
		Name:    "Item 2",
		Brand:   "Brand B",
		Price:   "$2.00",
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
		ID:      5,
		FlyerID: 300,
		Name:    "Item 1",
	}
	item2 := storage.FlyerItem{
		ID:      5, // same ID and FlyerID
		FlyerID: 300,
		Name:    "Item 2",
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
		ID:      10,
		FlyerID: 400,
		Name:    "First Item",
	}
	item2 := storage.FlyerItem{
		ID:      11,
		FlyerID: 400,
		Name:    "Second Item",
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

func TestListFlyerItems_StableOrder(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add a flyer
	flyer := storage.Flyer{
		ID:        700,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer for Stable Order",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	// Add items in specific order
	items := []storage.FlyerItem{
		{ID: 1, FlyerID: 700, Name: "Z Item"},
		{ID: 2, FlyerID: 700, Name: "A Item"},
		{ID: 3, FlyerID: 700, Name: "M Item"},
	}
	for _, item := range items {
		if err := fs.AddFlyerItem(ctx, item); err != nil {
			t.Fatalf("AddFlyerItem %d failed: %v", item.ID, err)
		}
	}

	// List items
	listed, err := fs.ListFlyerItems(ctx, flyer.ID)
	if err != nil {
		t.Fatalf("ListFlyerItems failed: %v", err)
	}
	if len(listed) != 3 {
		t.Errorf("expected 3 flyer items, got %d", len(listed))
		return
	}
	// Check insertion order is preserved
	if listed[0].Name != "Z Item" {
		t.Errorf("first item name = %q, want Z Item", listed[0].Name)
	}
	if listed[1].Name != "A Item" {
		t.Errorf("second item name = %q, want A Item", listed[1].Name)
	}
	if listed[2].Name != "M Item" {
		t.Errorf("third item name = %q, want M Item", listed[2].Name)
	}
}

func TestHasFlyerItem_PresentAndAbsent(t *testing.T) {
	fs, ctx := newTestStorage(t)

	// Add a flyer
	flyer := storage.Flyer{
		ID:        800,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-01-31T23:59:59Z"),
		Name:      "Flyer for HasItem Test",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, flyer); err != nil {
		t.Fatalf("AddFlyer failed: %v", err)
	}

	// Test absent item
	has, err := fs.HasFlyerItem(ctx, flyer.ID, 999)
	if err != nil {
		t.Fatalf("HasFlyerItem failed: %v", err)
	}
	if has {
		t.Error("HasFlyerItem for absent item should return false")
	}

	// Add an item
	item := storage.FlyerItem{
		ID:      42,
		FlyerID: 800,
		Name:    "Present Item",
	}
	if err := fs.AddFlyerItem(ctx, item); err != nil {
		t.Fatalf("AddFlyerItem failed: %v", err)
	}

	// Test present item
	has, err = fs.HasFlyerItem(ctx, flyer.ID, item.ID)
	if err != nil {
		t.Fatalf("HasFlyerItem failed: %v", err)
	}
	if !has {
		t.Error("HasFlyerItem for present item should return true")
	}
}

func TestPruneExpired_RemovesStaleAndKeepsCurrent(t *testing.T) {
	fs, ctx := newTestStorage(t)

	now := mustTime(t, "2024-06-15T12:00:00Z")

	// Add an expired flyer (ValidTo before now)
	expired := storage.Flyer{
		ID:        1,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-06-01T23:59:59Z"), // expired
		Name:      "Expired Flyer",
		Merchant:  "Test Merchant",
	}
	// Add a current flyer (ValidTo after now)
	current := storage.Flyer{
		ID:        2,
		ValidFrom: mustTime(t, "2024-06-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-06-30T23:59:59Z"), // current
		Name:      "Current Flyer",
		Merchant:  "Test Merchant",
	}
	// Add a flyer with zero ValidTo (should be kept - treat as no expiry)
	noExpiry := storage.Flyer{
		ID:        3,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		Name:      "No Expiry Flyer",
		Merchant:  "Test Merchant",
	}

	if err := fs.AddFlyer(ctx, expired); err != nil {
		t.Fatalf("AddFlyer expired failed: %v", err)
	}
	if err := fs.AddFlyer(ctx, current); err != nil {
		t.Fatalf("AddFlyer current failed: %v", err)
	}
	if err := fs.AddFlyer(ctx, noExpiry); err != nil {
		t.Fatalf("AddFlyer noExpiry failed: %v", err)
	}

	// Add items to each flyer to test cascade
	for _, f := range []storage.Flyer{expired, current, noExpiry} {
		item := storage.FlyerItem{
			ID:      1,
			FlyerID: f.ID,
			Name:    "Test Item",
		}
		if err := fs.AddFlyerItem(ctx, item); err != nil {
			t.Fatalf("AddFlyerItem for flyer %d failed: %v", f.ID, err)
		}
	}

	// Run prune
	if err := fs.PruneExpired(ctx, now); err != nil {
		t.Fatalf("PruneExpired failed: %v", err)
	}

	// Check expired flyer is removed
	hasExpired, err := fs.HasFlyer(ctx, expired.ID)
	if err != nil {
		t.Fatalf("HasFlyer for expired failed: %v", err)
	}
	if hasExpired {
		t.Error("Expired flyer should be removed")
	}

	// Check current flyer is kept
	hasCurrent, err := fs.HasFlyer(ctx, current.ID)
	if err != nil {
		t.Fatalf("HasFlyer for current failed: %v", err)
	}
	if !hasCurrent {
		t.Error("Current flyer should be kept")
	}

	// Check noExpiry flyer is kept
	hasNoExpiry, err := fs.HasFlyer(ctx, noExpiry.ID)
	if err != nil {
		t.Fatalf("HasFlyer for noExpiry failed: %v", err)
	}
	if !hasNoExpiry {
		t.Error("NoExpiry flyer should be kept")
	}

	// Check items cascade
	// Expired flyer items should be gone (returns empty slice, not error)
	expiredItems, err := fs.ListFlyerItems(ctx, expired.ID)
	if err != nil {
		t.Fatalf("ListFlyerItems for expired items failed unexpectedly: %v", err)
	}
	if len(expiredItems) != 0 {
		t.Errorf("expected 0 items for expired flyer (items removed), got %d", len(expiredItems))
	}

	// Current flyer items should remain
	currentItems, err := fs.ListFlyerItems(ctx, current.ID)
	if err != nil {
		t.Fatalf("ListFlyerItems for current failed: %v", err)
	}
	if len(currentItems) != 1 {
		t.Errorf("expected 1 item for current flyer, got %d", len(currentItems))
	}

	// NoExpiry flyer items should remain
	noExpiryItems, err := fs.ListFlyerItems(ctx, noExpiry.ID)
	if err != nil {
		t.Fatalf("ListFlyerItems for noExpiry failed: %v", err)
	}
	if len(noExpiryItems) != 1 {
		t.Errorf("expected 1 item for noExpiry flyer, got %d", len(noExpiryItems))
	}
}

func TestPruneExpired_Idempotent(t *testing.T) {
	fs, ctx := newTestStorage(t)

	now := mustTime(t, "2024-06-15T12:00:00Z")

	// Add an expired flyer
	expired := storage.Flyer{
		ID:        1,
		ValidFrom: mustTime(t, "2024-01-01T00:00:00Z"),
		ValidTo:   mustTime(t, "2024-06-01T23:59:59Z"), // expired
		Name:      "Expired Flyer",
		Merchant:  "Test Merchant",
	}
	if err := fs.AddFlyer(ctx, expired); err != nil {
		t.Fatalf("AddFlyer expired failed: %v", err)
	}

	// Run prune multiple times
	for i := 0; i < 3; i++ {
		if err := fs.PruneExpired(ctx, now); err != nil {
			t.Fatalf("PruneExpired() call %d failed: %v", i+1, err)
		}
	}

	// Verify flyer is gone
	has, err := fs.HasFlyer(ctx, expired.ID)
	if err != nil {
		t.Fatalf("HasFlyer failed: %v", err)
	}
	if has {
		t.Error("Expired flyer should be removed")
	}
}

func TestPruneExpired_NoFlyers(t *testing.T) {
	fs, ctx := newTestStorage(t)

	now := mustTime(t, "2024-06-15T12:00:00Z")

	// Should not error on empty store
	if err := fs.PruneExpired(ctx, now); err != nil {
		t.Fatalf("PruneExpired on empty store failed: %v", err)
	}
}

// Concurrency tests

func TestConcurrent_AddGrocery_NoTornWrites(t *testing.T) {
	fs, ctx := newTestStorage(t)

	const numGoroutines = 50
	// Pre-compute embeddings so mustEmbed (which can call t.Fatalf) is never
	// invoked from a non-test goroutine.
	embeddings := make([][]float32, numGoroutines)
	for i := range embeddings {
		embeddings[i] = mustEmbed(t, 3)
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Track which names we added
	addedNames := make(map[string]bool)
	var mu sync.Mutex

	// Launch goroutines to add unique items
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("item_%03d", id)
			item := storage.GroceryItem{
				Name:      name,
				Embedding: embeddings[id],
			}
			mu.Lock()
			addedNames[name] = true
			mu.Unlock()
			if err := fs.AddGrocery(ctx, item); err != nil {
				t.Errorf("AddGrocery %s failed: %v", name, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all items were added
	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != numGoroutines {
		t.Errorf("expected %d groceries after concurrent adds, got %d", numGoroutines, len(groceries))
		return
	}

	// Check that all expected names are present
	foundNames := make(map[string]bool)
	for _, g := range groceries {
		foundNames[g.Name] = true
	}
	for name := range addedNames {
		if !foundNames[name] {
			t.Errorf("expected grocery %s not found in results", name)
		}
	}
}

func TestConcurrent_ReadAndWrite_NoTornWrites(t *testing.T) {
	fs, ctx := newTestStorage(t)

	const numAdders = 25
	const numListers = 25
	var wg sync.WaitGroup
	wg.Add(numAdders + numListers)

	var addErrors, listErrors []string
	var mu sync.Mutex

	// Launch adders
	for i := 0; i < numAdders; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent_%d", id)
			item := storage.GroceryItem{
				Name:      name,
				Embedding: mustEmbed(t, 2),
			}
			if err := fs.AddGrocery(ctx, item); err != nil {
				mu.Lock()
				addErrors = append(addErrors, fmt.Sprintf("AddGrocery %s: %v", name, err))
				mu.Unlock()
			}
		}(i)
	}

	// Launch listers
	for i := 0; i < numListers; i++ {
		go func() {
			defer wg.Done()
			if _, err := fs.ListGroceries(ctx); err != nil {
				mu.Lock()
				listErrors = append(listErrors, fmt.Sprintf("ListGroceries: %v", err))
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(addErrors) > 0 {
		t.Errorf("errors during concurrent adds: %v", addErrors)
	}
	if len(listErrors) > 0 {
		t.Errorf("errors during concurrent lists: %v", listErrors)
	}

	// Verify all added items are present
	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != numAdders {
		t.Errorf("expected %d groceries after concurrent adds+listers, got %d", numAdders, len(groceries))
		return
	}
	for i := 0; i < numAdders; i++ {
		name := fmt.Sprintf("concurrent_%d", i)
		found := false
		for _, g := range groceries {
			if g.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected grocery %s not found in final list", name)
		}
	}
}

func TestClose_Idempotent(t *testing.T) {
	fs, _ := newTestStorage(t)

	// Should not error when called multiple times
	if err := fs.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("Close() second call failed: %v", err)
	}
}

// Concurrency: adding the same grocery name from many goroutines must yield
// exactly one success and one ErrDuplicate per loser, with no torn writes.
func TestConcurrent_AddSameGrocery_SingleSuccess(t *testing.T) {
	fs, ctx := newTestStorage(t)

	const numGoroutines = 20
	embed := mustEmbed(t, 3)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	var mu sync.Mutex
	successes := 0
	dupErrors := 0
	otherErrors := 0

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := fs.AddGrocery(ctx, storage.GroceryItem{Name: "milk", Embedding: embed})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successes++
			} else if errors.Is(err, storage.ErrDuplicate) {
				dupErrors++
			} else {
				otherErrors++
			}
		}()
	}

	wg.Wait()

	if successes != 1 {
		t.Errorf("expected exactly 1 success, got %d", successes)
	}
	if dupErrors != numGoroutines-1 {
		t.Errorf("expected %d ErrDuplicate errors, got %d", numGoroutines-1, dupErrors)
	}
	if otherErrors != 0 {
		t.Errorf("expected 0 other errors, got %d", otherErrors)
	}

	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != 1 {
		t.Errorf("expected 1 stored grocery, got %d", len(groceries))
	}
}

// Legacy camelCase flyers_index.json (the old RetailGroup on-disk schema) must be
// normalized to the canonical snake_case storage.Flyer form during Migrate.
func TestMigrate_LegacyCamelCaseFlyersIndex(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	legacy := []byte(`[{"id":1,"validFrom":"2024-01-01","validTo":"2024-01-31","name":"Test Flyer","merchant":"Test Merchant","aux":{"foo":"bar"},"location":[{"id":1,"address":"123 Main St","city":"Anytown","province":"ON","postalCode":"A1A 1A1"}]}]`)
	if err := os.WriteFile(filepath.Join(tmpDir, flyersIndex), legacy, 0o600); err != nil {
		t.Fatalf("failed to write legacy flyers_index: %v", err)
	}

	fs, err := NewJSONAt(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewJSONAt failed: %v", err)
	}
	defer fs.Close()

	rewritten, err := os.ReadFile(filepath.Join(tmpDir, flyersIndex))
	if err != nil {
		t.Fatalf("failed to read migrated flyers_index: %v", err)
	}
	if bytes.Contains(rewritten, []byte(`"validFrom"`)) || bytes.Contains(rewritten, []byte(`"location"`)) {
		t.Errorf("flyers_index.json still contains legacy keys: %s", rewritten)
	}

	flyers, err := fs.ListFlyers(ctx)
	if err != nil {
		t.Fatalf("ListFlyers failed: %v", err)
	}
	if len(flyers) != 1 {
		t.Fatalf("expected 1 flyer after migration, got %d", len(flyers))
	}
	f := flyers[0]
	if !f.ValidFrom.Equal(mustTime(t, "2024-01-01T00:00:00Z")) {
		t.Errorf("ValidFrom = %v, want 2024-01-01T00:00:00Z", f.ValidFrom)
	}
	if !f.ValidTo.Equal(mustTime(t, "2024-01-31T00:00:00Z")) {
		t.Errorf("ValidTo = %v, want 2024-01-31T00:00:00Z", f.ValidTo)
	}
	if f.Name != "Test Flyer" {
		t.Errorf("Name = %q, want %q", f.Name, "Test Flyer")
	}
	if f.Merchant != "Test Merchant" {
		t.Errorf("Merchant = %q, want %q", f.Merchant, "Test Merchant")
	}
	if len(f.Stores) != 1 {
		t.Fatalf("expected 1 store after migration, got %d", len(f.Stores))
	}
	if f.Stores[0].ID != 1 {
		t.Errorf("Store.ID = %d, want 1", f.Stores[0].ID)
	}
	if f.Stores[0].PostalCode != "A1A 1A1" {
		t.Errorf("Store.PostalCode = %q, want %q", f.Stores[0].PostalCode, "A1A 1A1")
	}

	// Idempotent: a second Migrate must be a no-op and not error.
	if err := fs.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}
	flyers2, err := fs.ListFlyers(ctx)
	if err != nil {
		t.Fatalf("ListFlyers after re-migrate failed: %v", err)
	}
	if len(flyers2) != 1 {
		t.Errorf("expected 1 flyer after re-migrate, got %d", len(flyers2))
	}
}

// Legacy groceries.json written as a bare []string (old db.SaveGroceries) must
// still load as GroceryItems without embeddings, and AddGrocery must keep working.
func TestLoadGroceries_LegacyStringArray(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	legacy := []byte(`["milk","bread"]`)
	if err := os.WriteFile(filepath.Join(tmpDir, groceriesFile), legacy, 0o600); err != nil {
		t.Fatalf("failed to write legacy groceries.json: %v", err)
	}

	fs, err := NewJSONAt(ctx, tmpDir)
	if err != nil {
		t.Fatalf("NewJSONAt failed: %v", err)
	}
	defer fs.Close()

	groceries, err := fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries failed: %v", err)
	}
	if len(groceries) != 2 {
		t.Fatalf("expected 2 groceries from legacy array, got %d", len(groceries))
	}
	if groceries[0].Name != "milk" || groceries[1].Name != "bread" {
		t.Errorf("grocery names = %q, %q; want milk, bread", groceries[0].Name, groceries[1].Name)
	}

	// AddGrocery must still work for a brand-new item; it rewrites the index in
	// canonical form, after which the legacy schema is gone for this file.
	if err := fs.AddGrocery(ctx, storage.GroceryItem{Name: "eggs", Embedding: mustEmbed(t, 3)}); err != nil {
		t.Fatalf("AddGrocery eggs failed: %v", err)
	}
	groceries, err = fs.ListGroceries(ctx)
	if err != nil {
		t.Fatalf("ListGroceries after add failed: %v", err)
	}
	if len(groceries) != 3 {
		t.Errorf("expected 3 groceries after adding eggs, got %d", len(groceries))
	}
}

// Helper for comparing float32 slices
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

// TestStorageContract runs the cross-backend parity suite against JSON.
func TestStorageContract(t *testing.T) {
	parity.TestStorageContract(t, func(t *testing.T) storage.Storage {
		fs, err := NewJSONAt(context.Background(), t.TempDir())
		if err != nil {
			t.Fatalf("NewJSONAt: %v", err)
		}
		return fs
	})
}
