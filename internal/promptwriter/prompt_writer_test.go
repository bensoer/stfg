package promptwriter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"stfg/internal/provider"
	"stfg/internal/storage"
)

// fakeProvider records each prompt and model passed to Send and returns a
// scripted response or error.
type fakeProvider struct {
	prompts   []string
	models    []string
	responses [][]provider.GroceryFlyerMatch
	errs      []error
	calls     int
}

func (f *fakeProvider) Send(_ context.Context, prompt string, model string) ([]provider.GroceryFlyerMatch, error) {
	idx := f.calls
	f.calls++
	f.prompts = append(f.prompts, prompt)
	f.models = append(f.models, model)
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.responses) {
		return f.responses[idx], nil
	}
	return nil, errors.New("fakeProvider: no scripted response")
}

func TestNewPromptWriter_RejectsNilProvider(t *testing.T) {
	if _, err := NewPromptWriter(nil, "openrouter/free"); err == nil {
		t.Fatal("expected error for nil provider, got nil")
	}
}

func TestNewPromptWriter_RejectsEmptyModel(t *testing.T) {
	fake := &fakeProvider{}
	if _, err := NewPromptWriter(fake, ""); err == nil {
		t.Fatal("expected error for empty model, got nil")
	}
}

func TestPromptWriter_ForwardsModelAndMatchesValidResponse(t *testing.T) {
	flyer := storage.FlyerItem{ID: 42, Name: "2% Milk"}
	fp := &fakeProvider{
		responses: [][]provider.GroceryFlyerMatch{
			{{GroceryItem: "milk", FlyerItemID: 42, FlyerItemName: "2% Milk"}},
		},
	}
	pw, err := NewPromptWriter(fp, "openrouter/free")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	matches, err := pw.GetFlyerItemsOnGroceryList(
		context.Background(),
		[]storage.FlyerItem{flyer},
		[]storage.GroceryItem{{Name: "milk"}},
	)
	if err != nil {
		t.Fatalf("unexpected method error: %v", err)
	}
	if fp.calls != 1 {
		t.Fatalf("expected Send to be called once, got %d", fp.calls)
	}
	if fp.models[0] != "openrouter/free" {
		t.Errorf("expected model %q, got %q", "openrouter/free", fp.models[0])
	}
	prompt := fp.prompts[0]
	if !strings.Contains(prompt, `"matches"`) {
		t.Errorf("expected prompt to contain the matches envelope example, got: %q", prompt)
	}
	if strings.Contains(prompt, "a JSON list of objects matching this spec") {
		t.Errorf("prompt should not use a bare JSON array as the response example")
	}
	hits, ok := matches["milk"]
	if !ok {
		t.Fatalf("expected match for grocery %q, got %v", "milk", matches)
	}
	if len(hits) != 1 || hits[0].ID != 42 || hits[0].Name != "2% Milk" {
		t.Errorf("unexpected flyer item mapping: %+v", hits)
	}
}

func TestPromptWriter_RetriesOnUnknownFlyerID(t *testing.T) {
	flyer := storage.FlyerItem{ID: 1, Name: "Real Item"}
	fp := &fakeProvider{
		responses: [][]provider.GroceryFlyerMatch{
			{{GroceryItem: "milk", FlyerItemID: 999, FlyerItemName: "Imaginary"}},
			{{GroceryItem: "milk", FlyerItemID: 999, FlyerItemName: "Imaginary"}},
			{{GroceryItem: "milk", FlyerItemID: 999, FlyerItemName: "Imaginary"}},
		},
	}
	pw, err := NewPromptWriter(fp, "openrouter/free")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	_, err = pw.GetFlyerItemsOnGroceryList(
		context.Background(),
		[]storage.FlyerItem{flyer},
		[]storage.GroceryItem{{Name: "milk"}},
	)
	if err == nil {
		t.Fatal("expected retry-exhaustion error, got nil")
	}
	if err.Error() != "Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer" {
		t.Errorf("unexpected error string: %q", err.Error())
	}
	if fp.calls != 3 {
		t.Fatalf("expected Send to be called 3 times, got %d", fp.calls)
	}
}

func TestPromptWriter_AbortsImmediatelyOnSendError(t *testing.T) {
	sendErr := errors.New("boom")
	fp := &fakeProvider{
		errs: []error{sendErr},
	}
	pw, err := NewPromptWriter(fp, "openrouter/free")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	_, err = pw.GetFlyerItemsOnGroceryList(
		context.Background(),
		[]storage.FlyerItem{{ID: 1, Name: "Milk"}},
		[]storage.GroceryItem{{Name: "milk"}},
	)
	if err == nil {
		t.Fatal("expected wrapped send error, got nil")
	}
	if !strings.Contains(err.Error(), "error sending prompt") {
		t.Errorf("expected error to be wrapped with 'error sending prompt', got: %q", err.Error())
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("expected wrapped error to unwrap to sendErr")
	}
	if fp.calls != 1 {
		t.Fatalf("expected Send to be called once, got %d", fp.calls)
	}
}
