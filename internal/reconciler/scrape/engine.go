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
// stored for flyerID. New items get an embedding; existing items get a fresh
// embedding only when the text changed (brand or name), the price, value, or
// any of the persisted media/display fields. Price-only changes do not call
// the embedder. Unchanged items are skipped entirely. Already-written rows
// stay on disk when the embed call fails — the next scrape will backfill
// their embeddings.
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
		existing, existed := byID[item.ID]

		if !existed {
			// New item: persist a nil embedding when the text is empty
			// (find-deals skips such items silently); otherwise embed once
			// and store the vector.
			if text != "" {
				embedding, err := embedder.CreateGroceryEmbedding(ctx, text)
				if err != nil {
					return fmt.Errorf("scrape: embed flyer %d item %d: %w", flyerID, item.ID, err)
				}
				item.Embedding = embedding
				embeddedCount++
			}
			if err := store.AddFlyerItem(ctx, item); err != nil {
				return err
			}
			continue
		}

		storedText := strings.TrimSpace(existing.Brand + " " + existing.Name)
		needsEmbedding := len(existing.Embedding) == 0 || storedText != text
		var embedding []float32
		switch {
		case needsEmbedding && text == "":
			// Empty text: keep the slot unembedded so find-deals knows to
			// skip this item without erroring.
		case needsEmbedding:
			embedding, err = embedder.CreateGroceryEmbedding(ctx, text)
			if err != nil {
				return fmt.Errorf("scrape: embed flyer %d item %d: %w", flyerID, item.ID, err)
			}
			embeddedCount++
		default:
			embedding = existing.Embedding
		}

		if !itemChanged(existing, item, embedding) {
			continue
		}
		item.Embedding = embedding
		if err := store.UpdateFlyerItem(ctx, item); err != nil {
			return err
		}
	}

	if embeddedCount > 0 {
		log.Infof("Flyer %d: embedded %d items", flyerID, embeddedCount)
	}
	return nil
}

// itemChanged reports whether item differs from existing on any column that
// UpdateFlyerItem persists. Price-only changes must trigger an update but
// must not trigger an embed (the caller already gates embedding separately).
func itemChanged(existing, item storage.FlyerItem, embedding []float32) bool {
	if existing.Name != item.Name {
		return true
	}
	if existing.Brand != item.Brand {
		return true
	}
	if existing.Price != item.Price {
		return true
	}
	if existing.ImageURL != item.ImageURL {
		return true
	}
	if existing.VideoURL != item.VideoURL {
		return true
	}
	if existing.DisplayType != item.DisplayType {
		return true
	}
	return !equalFloat32(existing.Embedding, embedding)
}

func equalFloat32(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FlyerIsValid checks whether a flyer's validity window contains the current time.
func FlyerIsValid(flyer storage.Flyer) bool {
	now := time.Now()
	return now.After(flyer.ValidFrom) && now.Before(flyer.ValidTo)
}
