package scrape

type ScrapeReconcilerOptions struct {
	PostalCode           string
	RetailGroupWhiteList []string
	// Embedder computes and stores the flyer-item embedding at scrape time.
	// The reconciler errors out before any I/O when Embedder is nil, so a
	// missing embedder fails loudly instead of producing persisted items
	// without vectors.
	Embedder Embedder
}
