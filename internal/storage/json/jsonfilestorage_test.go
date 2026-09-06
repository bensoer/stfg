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

