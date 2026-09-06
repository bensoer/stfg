package internal

import (
	"errors"
	"fmt"
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

// ParseDate parses a date string in RFC3339 or YYYY-MM-DD format.
func ParseDate(d string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, d)
	if err == nil {
		return t, nil
	}
	// Fallback: Flipp API may send dates in YYYY-MM-DD format
	t, err = time.Parse("2006-01-02", d)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q: not RFC3339 or YYYY-MM-DD", d)
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
