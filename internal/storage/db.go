package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func SaveJSON(filename string, data any) error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, filename)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	return enc.Encode(data)
}

func LoadJSON(filename string, out any) error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, out)
}

func FlyerFileName(flyerID int64) string {
	return fmt.Sprintf("flyer_%d.json", flyerID)
}

func FlyersIndexFile() string {
	return "flyers_index.json"
}
