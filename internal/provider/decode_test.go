package provider

import (
	"errors"
	"testing"
)

func TestDecodeGroceryFlyerMatches_SingleMatch(t *testing.T) {
	raw := `{"matches":[{"grocery_item":"milk","flyer_item_id":42,"flyer_item_name":"2% Milk"}]}`
	matches, err := decodeGroceryFlyerMatches("openai", raw)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].GroceryItem != "milk" {
		t.Errorf("expected GroceryItem \"milk\", got %q", matches[0].GroceryItem)
	}
	if matches[0].FlyerItemID != 42 {
		t.Errorf("expected FlyerItemID 42, got %d", matches[0].FlyerItemID)
	}
	if matches[0].FlyerItemName != "2% Milk" {
		t.Errorf("expected FlyerItemName \"2%% Milk\", got %q", matches[0].FlyerItemName)
	}
}

func TestDecodeGroceryFlyerMatches_EmptyArray(t *testing.T) {
	raw := `{"matches":[]}`
	matches, err := decodeGroceryFlyerMatches("openai", raw)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
	if matches == nil {
		t.Fatal("expected non-nil empty slice")
	}
}

func TestDecodeGroceryFlyerMatches_InvalidResponses(t *testing.T) {
	invalids := []string{
		`{`,
		`{}`,
		`{"matches":null}`,
	}
	for _, raw := range invalids {
		_, err := decodeGroceryFlyerMatches("openai", raw)
		if err == nil {
			t.Errorf("expected error for raw %q, got nil", raw)
			continue
		}
		var mre *ModelResponseError
		if !errors.As(err, &mre) {
			t.Errorf("expected *ModelResponseError for raw %q, got %T: %v", raw, err, err)
		}
	}
}
