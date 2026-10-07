package embedding

import (
	"context"
	"errors"
	"strings"
	"testing"

	"stfg/internal/provider"
)

// fakeProvider records each call to Embed and returns a scripted vector or
// error. Send must not be called from the embedder; reaching it is a test
// failure.
type fakeProvider struct {
	embedTexts   []string
	embedModels  []string
	embedCtxs    []context.Context
	embedVectors [][]float32
	embedErrs    []error
	embedCalls   int
}

func (f *fakeProvider) Send(_ context.Context, _ string, _ string) ([]provider.GroceryFlyerMatch, error) {
	return nil, errors.New("Send must not be called from the embedder")
}

func (f *fakeProvider) Embed(ctx context.Context, text string, model string) ([]float32, error) {
	idx := f.embedCalls
	f.embedCalls++
	f.embedTexts = append(f.embedTexts, text)
	f.embedModels = append(f.embedModels, model)
	f.embedCtxs = append(f.embedCtxs, ctx)
	if idx < len(f.embedErrs) && f.embedErrs[idx] != nil {
		return nil, f.embedErrs[idx]
	}
	if idx < len(f.embedVectors) {
		return f.embedVectors[idx], nil
	}
	return nil, errors.New("fakeProvider: no scripted embed")
}

func TestNewGroceryEmbedder_RejectsNilProvider(t *testing.T) {
	if _, err := NewGroceryEmbedder(nil, "test-model"); err == nil {
		t.Fatal("expected error for nil provider, got nil")
	}
}

func TestNewGroceryEmbedder_RejectsEmptyModel(t *testing.T) {
	fp := &fakeProvider{}
	if _, err := NewGroceryEmbedder(fp, ""); err == nil {
		t.Fatal("expected error for empty model, got nil")
	}
}

func TestGroceryEmbedder_RejectsEmptyGrocery(t *testing.T) {
	fp := &fakeProvider{}
	g, err := NewGroceryEmbedder(fp, "test-model")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	if _, err := g.CreateGroceryEmbedding(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty grocery, got nil")
	}
	if fp.embedCalls != 0 {
		t.Fatalf("Embed must not be called for an empty grocery, got %d calls", fp.embedCalls)
	}
}

func TestGroceryEmbedder_ForwardsToEmbed(t *testing.T) {
	want := []float32{0.25, -0.5, 1.0}
	fp := &fakeProvider{
		embedVectors: [][]float32{want},
	}
	g, err := NewGroceryEmbedder(fp, "test-model")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	got, err := g.CreateGroceryEmbedding(context.Background(), "milk")
	if err != nil {
		t.Fatalf("unexpected method error: %v", err)
	}
	if fp.embedCalls != 1 {
		t.Fatalf("expected Embed to be called once, got %d", fp.embedCalls)
	}
	if fp.embedTexts[0] != "milk" {
		t.Errorf("expected text %q, got %q", "milk", fp.embedTexts[0])
	}
	if fp.embedModels[0] != "test-model" {
		t.Errorf("expected model %q, got %q", "test-model", fp.embedModels[0])
	}
	if fp.embedCtxs[0] == nil {
		t.Error("expected a non-nil context to be passed to Embed")
	}
	if len(got) != len(want) {
		t.Fatalf("expected vector length %d, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected vector[%d] = %v, got %v", i, want[i], got[i])
		}
	}
}

func TestGroceryEmbedder_WrapsEmbedError(t *testing.T) {
	embedErr := errors.New("upstream failure")
	fp := &fakeProvider{
		embedErrs: []error{embedErr},
	}
	g, err := NewGroceryEmbedder(fp, "test-model")
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	_, err = g.CreateGroceryEmbedding(context.Background(), "milk")
	if err == nil {
		t.Fatal("expected wrapped error, got nil")
	}
	if !errors.Is(err, embedErr) {
		t.Errorf("expected error to unwrap to embedErr")
	}
	if !strings.Contains(err.Error(), "create grocery embedding") {
		t.Errorf("expected error to be wrapped with 'create grocery embedding', got: %q", err.Error())
	}
	if fp.embedCalls != 1 {
		t.Fatalf("expected Embed to be called once, got %d", fp.embedCalls)
	}
}
