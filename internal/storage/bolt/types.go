package bolt

// Options configures the BoltDB storage backend.
type BoltOptions struct {
	// CacheDir is the directory where the BoltDB database file is created.
	// If empty, storage.CacheDir() is used.
	CacheDir string
	// BoltFileName overrides the default database file name (stfg.bolt).
	// Mainly useful for tests.
	BoltFileName string
}
