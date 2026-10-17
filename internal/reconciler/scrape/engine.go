package scrape

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stfg/internal/flyerfinder"
	"stfg/internal/storage"

	"go.uber.org/zap"
)

func Reconcile(ctx context.Context, finder flyerfinder.FlyerFinder, store storage.StorageProvider, options ScrapeReconcilerOptions) error {
	log := zap.S()

	log.Info("Starting flyer reconciliation")

	if options.Embedder == nil {
		return errEmbedderMissing
	}

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

		isNewFlyer, err := reconcileFlyer(ctx, store, flyer)
		if err != nil {
			return err
		}

		if isNewFlyer {
			log.Infof("Adding Flyer \"%s - %s\" And Its Items", flyer.Merchant, flyer.Name)
		}

		items, err := finder.FindFlyerItems(flyer.ID)
		if err != nil {
			return err
		}

		if err := reconcileFlyerItems(ctx, store, options.Embedder, flyer.ID, items); err != nil {
			return err
		}
	}

	return nil
}

// reconcileFlyer inserts the flyer if it is not yet stored. The returned bool
// is true when the flyer was just added; an existing flyer is left untouched
// so its items can still be reconciled on the current run.
func reconcileFlyer(ctx context.Context, store storage.StorageProvider, flyer storage.Flyer) (bool, error) {
	has, err := store.HasFlyer(ctx, flyer.ID)
	if err != nil {
		return false, err
	}
	if has {
		return false, nil
	}
	if err := store.AddFlyer(ctx, flyer); err != nil {
		return false, err
	}
	return true, nil
}

// reconcileFlyerItems upserts each scraped item against the rows already
// stored for flyerID. New items are embedded and inserted; existing items
// are re-embedded and rewritten only when the brand-plus-name text differs
// from what is stored. Items are treated as immutable inside a flyer's
// validity window, so the text-differs path is the only re-embed trigger in
// practice — it covers the embedding-model-changed scenario after the
// operator wipes storage and re-scrapes. Items with empty brand and name are
// dropped before any I/O. An embed error aborts the scrape; already-written
// rows stay on disk.
func reconcileFlyerItems(
	ctx context.Context,
	store storage.StorageProvider,
	embedder Embedder,
	flyerID int64,
	scraped []storage.FlyerItem,
) error {
	log := zap.S()

	stored, err := store.ListFlyerItems(ctx, flyerID)
	if err != nil {
		return err
	}
	byID := make(map[int64]storage.FlyerItem, len(stored))
	for _, it := range stored {
		byID[it.ID] = it
	}

	var embeddedCount int
	for _, item := range scraped {
		text := strings.TrimSpace(item.Brand + " " + item.Name)
		if text == "" {
			continue
		}

		existing, existed := byID[item.ID]
		if !existed {
			embedding, err := embedder.CreateGroceryEmbedding(ctx, text)
			if err != nil {
				return fmt.Errorf("scrape: embed flyer %d item %d: %w", flyerID, item.ID, err)
			}
			item.Embedding = embedding
			if err := store.AddFlyerItem(ctx, item); err != nil {
				return err
			}
			embeddedCount++
			continue
		}

		storedText := strings.TrimSpace(existing.Brand + " " + existing.Name)
		if storedText == text {
			continue
		}
		embedding, err := embedder.CreateGroceryEmbedding(ctx, text)
		if err != nil {
			return fmt.Errorf("scrape: embed flyer %d item %d: %w", flyerID, item.ID, err)
		}
		item.Embedding = embedding
		if err := store.UpdateFlyerItem(ctx, item); err != nil {
			return err
		}
		embeddedCount++
	}

	if embeddedCount > 0 {
		log.Infof("Flyer %d: embedded %d items", flyerID, embeddedCount)
	}
	return nil
}

// FlyerIsValid checks whether a flyer's validity window contains the current time.
func FlyerIsValid(flyer storage.Flyer) bool {
	now := time.Now()
	return now.After(flyer.ValidFrom) && now.Before(flyer.ValidTo)
}
