package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

type JSONFileStorage struct {
}

const RetailGroupIndexFileName string = "flyers_index.json"
const GroceryFileName string = "groceries.json"

func NewJSONFileStorage() (*JSONFileStorage, error) {
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	// If path exists, already then we are good. Return object
	path := filepath.Join(dir, RetailGroupIndexFileName)
	if _, err := os.Stat(path); err != nil {
		// If path does not exist, create it
		err = SaveJSON(RetailGroupIndexFileName, []string{})
		if err != nil {
			return nil, err
		}
	}

	// Check the groceries.json file exists
	path = filepath.Join(dir, GroceryFileName)
	if _, err := os.Stat(path); err != nil {
		// If path does not exist, create it
		err = SaveJSON(GroceryFileName, []GroceryItem{})
		if err != nil {
			return nil, err
		}
	}

	return &JSONFileStorage{}, nil
}

func (j *JSONFileStorage) GetAllGroceries() ([]GroceryItem, error) {
	var g []GroceryItem
	err := j.loadJSON(GroceryFileName, &g)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (j *JSONFileStorage) AddGrocery(grocery GroceryItem) error {
	groceries, err := j.GetAllGroceries()
	if err != nil {
		return err
	}
	return j.saveJSON(GroceryFileName, append(groceries, grocery))
}

func (j *JSONFileStorage) RemoveGrocery(grocery GroceryItem) error {
	groceries, err := j.GetAllGroceries()
	if err != nil {
		return err
	}

	groceriesToKeep := []GroceryItem{}
	for _, existingGrocery := range groceries {
		if existingGrocery.Name != grocery.Name {
			groceriesToKeep = append(groceriesToKeep, existingGrocery)
		}
	}

	return j.saveJSON(GroceryFileName, groceriesToKeep)
}

func (j *JSONFileStorage) retailGroupFileName(flyerID int64) string {
	return fmt.Sprintf("flyer_%d.json", flyerID)
}

func (j *JSONFileStorage) retailGroupsIndexFile() string {
	return RetailGroupIndexFileName
}

func (j *JSONFileStorage) HasRetailGroup(flyer Flyer) bool {
	rtg, err := j.GetRetailGroup(flyer.ID)
	if err != nil {
		if errors.Is(err, RetailGroupNotFound) {
			return false
		}
		// Log the I/O error but treat as not-found for simplicity
		zap.S().Errorf("Error checking for retail group %d: %v", flyer.ID, err)
		return false
	}
	return rtg != nil
}

func (j *JSONFileStorage) HasRetailGroupItem(item FlyerItem) (bool, error) {

	items, err := j.GetRetailGroupItems(item.ID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// This means the file did not exist, so we can say it does not have a retail group item entry
			return false, nil
		}
		return false, err
	}

	for _, existing := range items {
		if existing.ID == item.ID {
			return true, nil
		}
	}

	return false, nil
}

func (j *JSONFileStorage) AddRetailGroup(flyer Flyer) error {

	var flyers []Flyer
	err := j.loadJSON(j.retailGroupsIndexFile(), &flyers)
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

	return j.saveJSON(j.retailGroupsIndexFile(), append(flyers, flyer))
}

func (j *JSONFileStorage) AddRetailGroupItem(item FlyerItem) error {
	items, err := j.GetRetailGroupItems(item.FlyerID)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return j.saveJSON(j.retailGroupFileName(item.FlyerID), append(items, item))
}

func (j *JSONFileStorage) GetRetailGroup(flyerID int64) (*Flyer, error) {
	var flyers []Flyer
	err := j.loadJSON(j.retailGroupsIndexFile(), &flyers)
	if err != nil {
		return nil, err
	}

	for i, f := range flyers {
		if f.ID == flyerID {
			return &flyers[i], nil
		}
	}

	return nil, RetailGroupNotFound
}

func (j *JSONFileStorage) RemoveRetailGroup(flyer Flyer) error {
	// If the group doesn't exist, then case closed
	if !j.HasRetailGroup(flyer) {
		return nil
	}

	// Assumes the flyer exists, so first need to get rid of any flyer items
	items, err := j.GetRetailGroupItems(flyer.ID)
	if err != nil {
		return err
	}
	for _, item := range items {
		j.RemoveRetailGroupItem(item)
	}

	// Now remove the flyer
	var flyers []Flyer
	err = j.loadJSON(j.retailGroupsIndexFile(), &flyers)
	if err != nil {
		return err
	}

	var flyersToKeep []Flyer
	for _, f := range flyers {
		// Keep everything BUT the flyer passed in
		if f.ID != flyer.ID {
			flyersToKeep = append(flyersToKeep, f)
		}
	}

	return j.saveJSON(j.retailGroupsIndexFile(), flyersToKeep)

}

func (j *JSONFileStorage) RemoveRetailGroupItem(item FlyerItem) error {
	items, err := j.GetRetailGroupItems(item.FlyerID)
	if err != nil {
		return err
	}

	itemsToKeep := []FlyerItem{}
	for _, existing := range items {
		if existing.ID != item.ID {
			itemsToKeep = append(itemsToKeep, existing)
		}
	}

	return j.saveJSON(j.retailGroupFileName(item.FlyerID), itemsToKeep)
}

func (j *JSONFileStorage) GetRetailGroupItems(flyerID int64) ([]FlyerItem, error) {

	var items []FlyerItem
	err := j.loadJSON(j.retailGroupFileName(flyerID), &items)
	if err != nil {
		return nil, err
	}
	return items, nil

}

func (j *JSONFileStorage) GetAllRetailGroups() ([]Flyer, error) {
	var flyers []Flyer
	err := j.loadJSON(j.retailGroupsIndexFile(), &flyers)
	if err != nil {
		return nil, err
	}
	return flyers, nil
}

func (j *JSONFileStorage) saveJSON(filename string, data any) error {
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

func (j *JSONFileStorage) loadJSON(filename string, out any) error {
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
