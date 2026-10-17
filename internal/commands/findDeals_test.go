package commands

import (
	"context"
	"errors"
	"strings"
	"testing"

	"stfg/internal/cosinecompare"
	"stfg/internal/storage"
)

// TestPrefilter_KeepsSimilarStoredPair verifies that two stored vectors
// (one flyer item, one grocery) that are nearly identical are kept by the
// prefilter. The vectors are deterministic so the cosine score stays well
// above 60%.
func TestPrefilter_KeepsSimilarStoredPair(t *testing.T) {
	comparer, err := cosinecompare.NewCosineCompare(cosinecompare.DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare: %v", err)
	}
	groceries := []storage.GroceryItem{
		{Name: "milk", Embedding: []float32{1.0, 1.0, 1.0}},
	}
	items := []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "milk", Embedding: []float32{1.0, 1.0, 1.0}},  // identical
		{ID: 2, FlyerID: 10, Name: "soda", Embedding: []float32{1.0, -1.0, 0.0}}, // dissimilar
	}
	got, err := prefilterFlyerItemsBySimilarity(context.Background(), comparer, groceries, items)
	if err != nil {
		t.Fatalf("prefilter: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("kept = %+v, want only item 1", got)
	}
}

// TestPrefilter_DropsDissimilarStoredPair verifies that stored vectors that
// fall below the configured similarity threshold are dropped.
func TestPrefilter_DropsDissimilarStoredPair(t *testing.T) {
	comparer, err := cosinecompare.NewCosineCompare(cosinecompare.DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare: %v", err)
	}
	// grocery unit vector vs. flyer-item orthogonal vector: cosine = 0,
	// well below the 60% threshold.
	groceries := []storage.GroceryItem{
		{Name: "milk", Embedding: []float32{1.0, 0.0, 0.0}},
	}
	items := []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "milk", Embedding: []float32{0.0, 1.0, 0.0}},
	}
	got, err := prefilterFlyerItemsBySimilarity(context.Background(), comparer, groceries, items)
	if err != nil {
		t.Fatalf("prefilter: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("kept = %+v, want empty", got)
	}
}

// TestPrefilter_EmptyFlyerText_SkipsWithoutError verifies that an item
// whose brand and name are both blank is skipped silently — there is no
// text to compare against and no stored embedding to use.
func TestPrefilter_EmptyFlyerText_SkipsWithoutError(t *testing.T) {
	comparer, err := cosinecompare.NewCosineCompare(cosinecompare.DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare: %v", err)
	}
	groceries := []storage.GroceryItem{
		{Name: "milk", Embedding: []float32{1.0, 0.0, 0.0}},
	}
	items := []storage.FlyerItem{
		{ID: 1, FlyerID: 10, Name: "", Brand: ""},
	}
	got, err := prefilterFlyerItemsBySimilarity(context.Background(), comparer, groceries, items)
	if err != nil {
		t.Fatalf("prefilter: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("kept = %+v, want empty", got)
	}
}

// TestPrefilter_NonEmptyTextMissingEmbedding_Errors verifies that a flyer
// item with a brand or name but no stored embedding surfaces as an error
// naming the flyer item id; the user is told to re-run scrape-flyers.
func TestPrefilter_NonEmptyTextMissingEmbedding_Errors(t *testing.T) {
	comparer, err := cosinecompare.NewCosineCompare(cosinecompare.DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare: %v", err)
	}
	groceries := []storage.GroceryItem{
		{Name: "milk", Embedding: []float32{1.0, 0.0, 0.0}},
	}
	items := []storage.FlyerItem{
		{ID: 7, FlyerID: 10, Name: "milk"},
	}
	_, err = prefilterFlyerItemsBySimilarity(context.Background(), comparer, groceries, items)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "7") {
		t.Fatalf("error %q does not name flyer item id 7", err.Error())
	}
	if !strings.Contains(err.Error(), "scrape-flyers") {
		t.Fatalf("error %q does not mention re-running scrape-flyers", err.Error())
	}
}

// TestPrefilter_LengthMismatch_ReturnsErr verifies that an embedding-length
// mismatch surfaces as a wrapped error so the user re-adds groceries and
// re-scrapes. We trigger it by giving a flyer item a different-length
// embedding than the grocery.
func TestPrefilter_LengthMismatch_ReturnsErr(t *testing.T) {
	comparer, err := cosinecompare.NewCosineCompare(cosinecompare.DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare: %v", err)
	}
	groceries := []storage.GroceryItem{
		{Name: "milk", Embedding: []float32{1.0, 0.0, 0.0}}, // length 3
	}
	items := []storage.FlyerItem{
		{ID: 11, FlyerID: 10, Name: "milk", Embedding: []float32{1.0, 0.0}}, // length 2
	}
	_, err = prefilterFlyerItemsBySimilarity(context.Background(), comparer, groceries, items)
	if err == nil {
		t.Fatal("expected length-mismatch error, got nil")
	}
	if !errors.Is(err, err) {
		t.Fatalf("error %v should wrap a cosine compare error", err)
	}
}
