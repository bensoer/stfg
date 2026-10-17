package scrape

import "errors"

// Sentinel errors. Callers in this package wrap these with errors.Is-compatible
// errors so the caller of Reconcile can match on a specific failure mode.
var (
	// errEmbedderMissing is returned when Reconcile is called without an
	// Embedder on the options struct. The reconciler needs the embedder to
	// write flyer-item vectors that find-deals later compares against grocery
	// embeddings; running without it would silently persist items with no
	// vector, which find-deals then surfaces as an error per item.
	errEmbedderMissing = errors.New("scrape: embedder is required on ScrapeReconcilerOptions")
)