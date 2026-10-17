package scrape

import "context"

// Embedder is the consumer-side interface the reconciler uses to embed the
// text it would otherwise drop. *embedding.GroceryEmbedder satisfies it
// directly; the reconciler does not need a wrapper. Methods take a
// context.Context so callers can cancel or time the embed call alongside the
// surrounding scrape.
type Embedder interface {
	CreateGroceryEmbedding(ctx context.Context, text string) ([]float32, error)
}
