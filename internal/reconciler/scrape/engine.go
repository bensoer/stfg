package scrape

import (
	"context"
	"strings"
	"time"

	"stfg/internal/storage"

	"go.uber.org/zap"
)

func Reconcile(ctx context.Context, scrapeClient ScrapeClient, storageClient storage.Storage, options ScrapeReconcilerOptions) error {
	log := zap.S()

	log.Info("Starting flyer reconciliation")

	if err := storageClient.PruneExpired(ctx, time.Now()); err != nil {
		return err
	}

	flyers, err := scrapeClient.GetRetailGroups(options.PostalCode)
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
		has, err := storageClient.HasFlyer(ctx, flyer.ID)
		if err != nil {
			return err
		}
		if has {
			continue
		}

		log.Infof("Adding Flyer \"%s - %s\" And Its Items", flyer.Merchant, flyer.Name)
		if err := storageClient.AddFlyer(ctx, flyer); err != nil {
			return err
		}

		items, err := scrapeClient.GetRetailGroupItems(flyer.ID)
		if err != nil {
			return err
		}

		for _, item := range items {
			hasItem, err := storageClient.HasFlyerItem(ctx, flyer.ID, item.ID)
			if err != nil {
				return err
			}
			if !hasItem {
				if err := storageClient.AddFlyerItem(ctx, item); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
