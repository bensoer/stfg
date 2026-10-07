package sqlite

// Options configures the SQLite storage backend.
type SQLiteOptions struct {
	// CacheDir is the directory where the SQLite database file is created.
	// If empty, storage.CacheDir() is used.
	CacheDir string
	// SQLiteFileName overrides the default database file name (stfg.sqlite).
	// Mainly useful for tests.
	SQLiteFileName string
}
