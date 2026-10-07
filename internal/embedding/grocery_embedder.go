// Package embedding generates vector representations of grocery items by
// delegating to a provider.Provider. The previous llama.cpp path is gone; the
// only embedding call in the program now goes through a hosted model.
package embedding

import (
	"context"
	"fmt"

	"stfg/internal/provider"
)

// GroceryEmbedder returns one float32 embedding for one grocery string by
// calling a configured provider.Provider.
type GroceryEmbedder struct {
	// provider is the LLM backend that produces the vector.
	provider provider.Provider
	// model is the model name passed to the provider.
	model string
}

// NewGroceryEmbedder constructs a GroceryEmbedder bound to the given provider
// and model. It returns an error if the provider is nil or the model is empty.
func NewGroceryEmbedder(p provider.Provider, model string) (*GroceryEmbedder, error) {
	if p == nil {
		return nil, fmt.Errorf("grocery embedder: provider is nil")
	}
	if model == "" {
		return nil, fmt.Errorf("grocery embedder: model is empty")
	}
	return &GroceryEmbedder{provider: p, model: model}, nil
}

// CreateGroceryEmbedding returns one float32 embedding for grocery.
func (g *GroceryEmbedder) CreateGroceryEmbedding(ctx context.Context, grocery string) ([]float32, error) {
	if grocery == "" {
		return nil, fmt.Errorf("grocery embedder: grocery is empty")
	}

	// 32768 is Qwen's max, but since were only generating embeddings for couple words up to a sentence.
	// 1024 is plenty. This also should save on users memory usage
	// Create context with embedding support
	//fmt.Printf("Model loaded successfully.\n")
	//fmt.Printf("Getting embeddings for: %s\n", *text)
	// Check if models/${modelName}.gguf exists
	// return the absolute path
	// Ensure destination directory exists
	// Check if partial file exists
	// Resume download
	// If server ignored Range request, restart
	embedding, err := g.provider.Embed(ctx, grocery, g.model)
	if err != nil {
		return nil, fmt.Errorf("create grocery embedding: %w", err)
	}
	return embedding, nil
}
