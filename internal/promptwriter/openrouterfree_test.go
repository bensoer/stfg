package promptwriter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"stfg/internal/provider"
	"stfg/internal/storage"
)

type fakePrompter struct {
	prompts   []string
	response  []provider.GroceryFlyerMatch
	err       error
	callCount int
}

func (f *fakePrompter) Send(_ context.Context, prompt, _ string) ([]provider.GroceryFlyerMatch, error) {
	f.callCount++
	f.prompts = append(f.prompts, prompt)
	return f.response, f.err
}

func TestGetFlyerItemsOnGroceryList_MatchFound(t *testing.T) {
	flyerItems := []storage.FlyerItem{
		{ID: 42, Name: "2% Milk 4L"},
	}
	groceries := []storage.GroceryItem{
		{Name: "milk"},
	}
	fake := &fakePrompter{
		response: []provider.GroceryFlyerMatch{
			{GroceryItem: "milk", FlyerItemID: 42, FlyerItemName: "2% Milk 4L"},
		},
	}
	writer := NewOpenRouterFreePromptWriter(fake)

	result, err := writer.GetFlyerItemsOnGroceryList(context.Background(), flyerItems, groceries)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if fake.callCount != 1 {
		t.Errorf("expected fake called once, got %d", fake.callCount)
	}
	items, ok := result["milk"]
	if !ok {
		t.Fatal("expected result keyed by \"milk\"")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 flyer item, got %d", len(items))
	}
	if items[0].ID != 42 || items[0].Name != "2% Milk 4L" {
		t.Errorf("expected flyer item {ID:42, Name:\"2%% Milk 4L\"}, got %+v", items[0])
	}

	prompt := fake.prompts[0]
	if !strings.Contains(prompt, "\"matches\"") {
		t.Error("expected prompt to contain \"matches\" envelope key")
	}
	if strings.Contains(prompt, "matching this spec:\n[") {
		t.Error("expected prompt to not use a bare JSON array response example")
	}
}

func TestGetFlyerItemsOnGroceryList_RetriesThenFails(t *testing.T) {
	flyerItems := []storage.FlyerItem{
		{ID: 42, Name: "2% Milk 4L"},
	}
	groceries := []storage.GroceryItem{
		{Name: "milk"},
	}
	fake := &fakePrompter{
		response: []provider.GroceryFlyerMatch{
			{GroceryItem: "milk", FlyerItemID: 999, FlyerItemName: "Nonexistent Item"},
		},
	}
	writer := NewOpenRouterFreePromptWriter(fake)

	_, err := writer.GetFlyerItemsOnGroceryList(context.Background(), flyerItems, groceries)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expectedErr := "Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
	if fake.callCount != 3 {
		t.Errorf("expected fake called 3 times, got %d", fake.callCount)
	}
}

func TestGetFlyerItemsOnGroceryList_SendError(t *testing.T) {
	flyerItems := []storage.FlyerItem{
		{ID: 42, Name: "2% Milk 4L"},
	}
	groceries := []storage.GroceryItem{
		{Name: "milk"},
	}
	fake := &fakePrompter{
		err: errors.New("network down"),
	}
	writer := NewOpenRouterFreePromptWriter(fake)

	_, err := writer.GetFlyerItemsOnGroceryList(context.Background(), flyerItems, groceries)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "error sending prompt") {
		t.Errorf("expected error containing \"error sending prompt\", got %q", err.Error())
	}
	if fake.callCount != 1 {
		t.Errorf("expected fake called once, got %d", fake.callCount)
	}
}
