package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"stfg/internal"
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

func GroceryFile() string {
	return "groceries.json"
}

func EnsureGroceryFileExists() error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, GroceryFile())

	if _, err := os.Stat(path); err == nil {
		return nil
	}

	return SaveJSON(GroceryFile(), []string{})
}

func LoadGroceries() ([]string, error) {
	var groceries []string
	err := LoadJSON(GroceryFile(), &groceries)
	if err != nil {
		return nil, err
	}
	return groceries, nil
}

func SaveGroceries(groceries []string) error {
	return SaveJSON(GroceryFile(), groceries)
}

func FlyerFileName(flyerID int64) string {
	return fmt.Sprintf("flyer_%d.json", flyerID)
}

func FlyersIndexFile() string {
	return "flyers_index.json"
}

func EnsureFlyerIndexFileExists() error {
	path := FlyersIndexFile()
	if !internal.FileExists(path) {
		if err := SaveJSON(path, []Flyer{}); err != nil {
			return err
		}
	}
	return nil
}

func RemoveInvalidFlyers() error {
	var flyers []Flyer
	err := LoadJSON(FlyersIndexFile(), &flyers)
	if err != nil {
		return err
	}

	validFlyers := []Flyer{}
	for _, f := range flyers {
		if f.ValidTo.IsZero() || !f.ValidTo.Before(time.Now()) {
			validFlyers = append(validFlyers, f)
		} else {
			// delete the flyer items file for this flyer
			if err := DeleteFlyerItems(f.ID); err != nil {
				return err
			}
		}
	}

	return SaveJSON(FlyersIndexFile(), validFlyers)
}

func SaveFlyer(flyer Flyer) error {
	// Load the flyers_index.json
	// Remove any flyer with the same ID
	// Append this new flyer to the list
	// Save the updated list back to flyers_index.json

	var flyers []Flyer
	err := LoadJSON(FlyersIndexFile(), &flyers)
	if err != nil {
		return err
	}

	// Remove any flyer with the same ID
	for i, f := range flyers {
		if f.ID == flyer.ID {
			flyers = append(flyers[:i], flyers[i+1:]...)
			break
		}
	}

	flyers = append(flyers, flyer)

	return SaveJSON(FlyersIndexFile(), flyers)
}

func LoadAllFlyers() ([]Flyer, error) {
	var flyers []Flyer
	err := LoadJSON(FlyersIndexFile(), &flyers)
	if err != nil {
		return nil, err
	}
	return flyers, nil
}

func LoadFlyer(flyerID int64) (*Flyer, error) {
	var flyers []Flyer
	err := LoadJSON(FlyersIndexFile(), &flyers)
	if err != nil {
		return nil, err
	}

	for _, f := range flyers {
		if f.ID == flyerID {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("flyer with ID %d not found", flyerID)
}

func SaveFlyerItems(flyerID int64, items []FlyerItem) error {
	return SaveJSON(FlyerFileName(flyerID), items)
}

func LoadFlyerItems(flyerID int64) ([]FlyerItem, error) {
	var items []FlyerItem
	err := LoadJSON(FlyerFileName(flyerID), &items)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func DeleteFlyerItems(flyerID int64) error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, FlyerFileName(flyerID))

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to delete
	}

	return os.Remove(path)
}
