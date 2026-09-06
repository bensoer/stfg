package scrape

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stfg/internal"
	"stfg/internal/flyerfinder"
	"stfg/internal/storage"

	"go.uber.org/zap"
)

func Reconcile(ctx context.Context, finder flyerfinder.FlyerFinder, store *storage.JSONFileStorage, options ScrapeReconcilerOptions) error {
	log := zap.S() //.With("source", "ScrapeReconciler")

	// iterate over store Flyers and remove whats invalid
	flyers, err := store.GetAllRetailGroups()
	if err != nil {
		return err
	}
	for _, f := range flyers {
		flyerIsValid, err := FlyerIsValid(f)
		if err != nil {
			return err
		}
		if !flyerIsValid {

			log.Infof("Flyer %s - %s Is Invalid. Removing", f.Merchant, f.Name)
			log.Infof("Removing All Flyer Items For Invalid Flyer %s - %s", f.Merchant, f.Name)
			items, err := store.GetRetailGroupItems(f.ID)
			if err != nil {
				return err
			}
			for _, item := range items {
				store.RemoveRetailGroupItem(item)
			}
			err = store.RemoveRetailGroup(f)
			if err != nil {
				return err
			}

		}
	}
	// All expired Flyers are now gone, so now we can add new stuff

	// iterate over flyers
	flyers, err = finder.FindFlyers(options.PostalCode)
	if err != nil {
		return err
	}

	for _, flyer := range flyers {

		// Validation check that we aren't being given invalid flyers from the scrapClient
		flyerIsValid, err := FlyerIsValid(flyer)
		if err != nil {
			return err
		}
		if !flyerIsValid {
			continue
		}

		// Filter only Flyers that we want to see
		found := false
		for _, whitelist := range options.RetailGroupWhiteList {
			fullName := fmt.Sprintf("%s %s", flyer.Merchant, flyer.Name)
			if strings.Contains(fullName, whitelist) {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// NOTE: We assume that no FlyerItems can have different validity timelines then the parent Flyer
		// All FlyerItems are valid within the Flyers start and end dates.

		// At this point we can gaurantee the flyer does not exist OR the flyer exists and is still valid

		// If it does not exist, add it!
		if !store.HasRetailGroup(flyer) {
			log.Infof("Adding Flyer \"%s - %s\" And Its Items", flyer.Merchant, flyer.Name)
			err := store.AddRetailGroup(flyer)
			if err != nil {
				return err
			}

			items, err := finder.FindFlyerItems(flyer.ID)
			if err != nil {
				return err
			}

			for _, item := range items {
				hasRetailGroupItem, err := store.HasRetailGroupItem(item)
				if err != nil {
					return err
				}
				if !hasRetailGroupItem {
					err := store.AddRetailGroupItem(item)
					if err != nil {
						return err
					}
				}
			}
		}

		// If it does already exist. Then we do nothing. Assuming nothing has changed thus

	}

	return nil
}

func FlyerIsValid(flyer storage.Flyer) (bool, error) {
	validFrom, err := internal.ParseDate(flyer.ValidFrom)
	if err != nil {
		return false, err
	}

	validTo, err := internal.ParseDate(flyer.ValidTo)
	if err != nil {
		return false, err
	}

	now := time.Now()
	if now.After(validFrom) && now.Before(validTo) {
		return true, nil
	} else {
		return false, nil
	}
}
