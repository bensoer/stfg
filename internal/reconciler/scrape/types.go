package scrape

import (
	"stfg/internal/storage"
)

type ScrapeReconcilerOptions struct {
	PostalCode           string
	RetailGroupWhiteList []string
}

type ScrapeClient interface {
	GetRetailGroups(postalCode string) ([]storage.Flyer, error)
	GetRetailGroupItems(retailGroupId int64) ([]storage.FlyerItem, error)
}

type RetailGroupLocation struct {
	ID         int    `json:"id"`
	Address    string `json:"address"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postalCode"`
}
