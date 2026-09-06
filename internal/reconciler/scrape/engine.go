package scrape

import (
	"context"
	"strings"
	"time"

	"stfg/internal/flyerfinder"
	"stfg/internal/storage"

	"go.uber.org/zap"
)

func Reconcile(ctx context.Context, finder flyerfinder.FlyerFinder, store storage.Storage, options ScrapeReconcilerOptions) error {
	log := zap.S()

	log.Info("Starting flyer reconciliation")

	if err := store.PruneExpired(ctx, time.Now()); err != nil {
		return err
	}

	flyers, err := finder.FindFlyers(options.PostalCode)
	if err != nil {
		return err
	}

	for _, flyer := range flyers {
		// Filter only flyers that we want to see
		fullName := flyer.Merchant + " " + flyer.Name
		found := false
		for _, whitelist := range options.RetailGroupWhiteList {
			if strings.Contains(fullName, whitelist) {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// If the flyer already exists, there is nothing to do
		has, err := store.HasFlyer(ctx, flyer.ID)
		if err != nil {
			return err
		}
		if has {
			continue
		}

		log.Infof("Adding Flyer \"%s - %s\" And Its Items", flyer.Merchant, flyer.Name)
		if err := store.AddFlyer(ctx, flyer); err != nil {
			return err
		}

		items, err := finder.FindFlyerItems(flyer.ID)
		if err != nil {
			return err
		}

		for _, item := range items {
			hasItem, err := store.HasFlyerItem(ctx, flyer.ID, item.ID)
			if err != nil {
				return err
			}
			if !hasItem {
				if err := store.AddFlyerItem(ctx, item); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// FlyerIsValid checks whether a flyer's validity window contains the current time.
func FlyerIsValid(flyer storage.Flyer) bool {
	now := time.Now()
	return now.After(flyer.ValidFrom) && now.Before(flyer.ValidTo)
}
