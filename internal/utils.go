package internal

import (
	"errors"
	"os"
	"time"
)

func Contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func ParseDate(d string) (time.Time, error) {
	return time.Parse(time.RFC3339, d)
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true // File exists
	}
	if errors.Is(err, os.ErrNotExist) {
		return false // File explicitly does not exist
	}
	// File may or may not exist (e.g., permission denied, broken symlink)
	return false
}
