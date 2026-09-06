package scrape

import (
	"stfg/internal/storage"
)

type ScrapeReconcilerOptions struct {
	PostalCode           string
	RetailGroupWhiteList []string
}

type ScrapeClient interface {
	GetFlyers(postalCode string) ([]storage.Flyer, error)
	GetFlyerItems(flyerID int64) ([]storage.FlyerItem, error)
}
