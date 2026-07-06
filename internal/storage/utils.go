package storage

import (
	"os"
	"path/filepath"
)

const AppName = "stfg"

func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, AppName)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return dir, nil
}
