package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"stfg/internal/reconciler/scrape"
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

func (j *JSONFileStorage) HasRetailGroup(retailGroup scrape.RetailGroup) bool {
	rtg, err := j.GetRetailGroup(retailGroup.ID)
	if err != nil || (err == nil && rtg == nil) {
		return false
	}
	return true
}

func (j *JSONFileStorage) HasRetailGroupItem(retailGroupItem scrape.RetailGroupItem) (bool, error) {

	rtgis, err := j.GetRetailGroupItems(retailGroupItem.ID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// This means the file did not exist, so we can say it does not have a retail group item entry
			return false, nil
		}
		return false, err
	}

	for _, rtgi := range rtgis {
		if rtgi.ID == retailGroupItem.ID {
			return true, nil
		}
	}

	return false, nil
}

func (j *JSONFileStorage) AddRetailGroup(retailGroup scrape.RetailGroup) error {

	var rtgs []scrape.RetailGroup
	err := j.loadJSON(j.retailGroupsIndexFile(), &rtgs)
	if err != nil {
		return err
	}

	// Remove any flyer with the same ID
	for i, f := range rtgs {
		if f.ID == retailGroup.ID {
			rtgs = append(rtgs[:i], rtgs[i+1:]...)
			break
		}
	}

	return j.saveJSON(j.retailGroupsIndexFile(), append(rtgs, retailGroup))
}

func (j *JSONFileStorage) AddRetailGroupItem(retailGroupItem scrape.RetailGroupItem) error {
	rtgis, err := j.GetRetailGroupItems(retailGroupItem.RetailGroupId)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return j.saveJSON(j.retailGroupFileName(retailGroupItem.RetailGroupId), append(rtgis, retailGroupItem))
}

func (j *JSONFileStorage) GetRetailGroup(retailGroupId int64) (*scrape.RetailGroup, error) {
	var rtgs []scrape.RetailGroup
	err := j.loadJSON(j.retailGroupsIndexFile(), &rtgs)
	if err != nil {
		return nil, err
	}

	for _, rtg := range rtgs {
		if rtg.ID == retailGroupId {
			return &rtg, nil
		}
	}

	return nil, RetailGroupNotFound
}

func (j *JSONFileStorage) RemoveRetailGroup(retailGroup scrape.RetailGroup) error {
	// If the group doesn't exist, then case closed
	if !j.HasRetailGroup(retailGroup) {
		return nil
	}

	// Assumes the retailgroup exists, so first need to get rid of any retail group items
	rtgis, err := j.GetRetailGroupItems(retailGroup.ID)
	if err != nil {
		return err
	}
	for _, rtgi := range rtgis {
		j.RemoveRetailGroupItem(rtgi)
	}

	// Now remove the retail group
	var rtgs []scrape.RetailGroup
	err = j.loadJSON(j.retailGroupsIndexFile(), &rtgs)
	if err != nil {
		return err
	}

	var rtgsToKeep []scrape.RetailGroup
	for _, rtg := range rtgs {
		// Keep everything BUT the retailGroup passed in
		if rtg.ID != retailGroup.ID {
			rtgsToKeep = append(rtgsToKeep, rtg)
		}
	}

	return j.saveJSON(j.retailGroupsIndexFile(), rtgsToKeep)

}

func (j *JSONFileStorage) RemoveRetailGroupItem(retailGroupItem scrape.RetailGroupItem) error {
	rtgis, err := j.GetRetailGroupItems(retailGroupItem.RetailGroupId)
	if err != nil {
		return err
	}

	rtgisToKeep := []scrape.RetailGroupItem{}
	for _, rtgi := range rtgis {
		if rtgi.ID != retailGroupItem.ID {
			rtgisToKeep = append(rtgisToKeep, rtgi)
		}
	}

	return j.saveJSON(j.retailGroupFileName(retailGroupItem.RetailGroupId), rtgisToKeep)
}

func (j *JSONFileStorage) GetRetailGroupItems(retailGroupId int64) ([]scrape.RetailGroupItem, error) {

	var rtgis []scrape.RetailGroupItem
	err := j.loadJSON(j.retailGroupFileName(retailGroupId), &rtgis)
	if err != nil {
		return nil, err
	}
	return rtgis, nil

}

func (j *JSONFileStorage) GetAllRetailGroups() ([]scrape.RetailGroup, error) {
	var rtgs []scrape.RetailGroup
	err := j.loadJSON(j.retailGroupsIndexFile(), &rtgs)
	if err != nil {
		return nil, err
	}
	return rtgs, nil
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
