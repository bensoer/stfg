package flyerfinder

import (
	"stfg/internal/storage"
)

// FlyerFinder defines the interface for finding flyers and flyer items.
type FlyerFinder interface {
	// FindFlyers returns a slice of flyers for the given postal code.
	FindFlyers(postalCode string) ([]storage.Flyer, error)
	// FindFlyerItems returns a slice of flyer items for the given flyer ID.
	FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error)
}
