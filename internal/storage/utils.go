package storage

import (
	"os"
	"path/filepath"
	"strings"
)

const AppName = "stfg"

func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, AppName)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	return dir, nil
}

// NormalizeName lowercases a grocery name for case-insensitive keying.
// Backends use this instead of strings.EqualFold so all three backends
// (JSON, sqlite, bolt) share the same normalization semantics. Under
// strings.EqualFold, certain Unicode pairs (e.g. "Σ"/"ς") are treated as
// duplicates, whereas strings.ToLower treats them as distinct — the shared
// ToLower semantics are what the parity contract pins.
func NormalizeName(name string) string {
	return strings.ToLower(name)
}
