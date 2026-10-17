package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"stfg/internal/container"
	"stfg/internal/cosinecompare"
	"stfg/internal/promptwriter"
	"stfg/internal/storage"
)

func FindDealsCmd(container *container.ContainerRegistry) *cobra.Command {

	// FindDealsCmd represents the find-deals command
	return &cobra.Command{
		Use:   "find-deals",
		Short: "Find deals in flyers matching your grocery list",
		Long:  `Search through downloaded flyers to find items from your grocery list that are on sale.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			store, err := container.GetStorage(ctx, "json")
			if err != nil {
				return fmt.Errorf("Error initializing storage: %w", err)
			}

			provider, err := container.GetProvider(ctx, "openrouter")
			if err != nil {
				return fmt.Errorf("Error initializing provider: %w", err)
			}

			flyers, err := store.ListFlyers(ctx)
			if err != nil {
				return fmt.Errorf("Error Loading Flyers: %w", err)
			}

			if len(flyers) == 0 {
				fmt.Fprintln(out, "There Are No Flyers Loaded. Have You Scraped First ?")
				return nil
			}

			groceries, err := store.ListGroceries(ctx)
			if err != nil {
				return fmt.Errorf("Error loading groceries: %w", err)
			}

			if len(groceries) == 0 {
				fmt.Fprintln(out, "Your grocery list is empty. Please add some items first.")
				return nil
			}

			// find-deals is a compare step, not an embedding step. Both the
			// grocery and the flyer-item vectors come from storage: groceries
			// were embedded when the user ran `groceries add-grocery`, and
			// flyer items were embedded when the user ran `scrape-flyers`. If
			// either side is missing, the run stops with a remediation
			// message; we never re-embed here.
			goodGroceries := make([]storage.GroceryItem, 0, len(groceries))
			for _, g := range groceries {
				if len(g.Embedding) == 0 {
					return fmt.Errorf("grocery %q has no embedding stored; re-add it with `groceries add-grocery` to generate one", g.Name)
				}
				goodGroceries = append(goodGroceries, g)
			}

			comparer, err := cosinecompare.NewCosineCompare(cosinecompare.DefaultSimilarityPercent)
			if err != nil {
				return fmt.Errorf("find deals: create cosine compare: %w", err)
			}

			promptWriter, err := promptwriter.NewPromptWriter(provider, "openai/gpt-4.1-nano")
			if err != nil {
				return fmt.Errorf("Error creating prompt writer: %w", err)
			}

			zap.S().Infof("Searching for deals on %d grocery items across %d flyers...", len(groceries), len(flyers))

			totalDeals := 0
			for _, flyer := range flyers {
				flyerItems, err := store.ListFlyerItems(ctx, flyer.ID)
				if err != nil {
					zap.S().Warnf("Error loading flyer %d: %v", flyer.ID, err)
					continue
				}

				if len(flyerItems) == 0 {
					continue
				}

				filteredItems, err := prefilterFlyerItemsBySimilarity(ctx, comparer, goodGroceries, flyerItems)
				if err != nil {
					return err
				}

				if len(filteredItems) == 0 {
					zap.S().Debugf("Flyer %d (%s): all items filtered out by cosine similarity prefilter", flyer.ID, flyer.Name)
					continue
				}

				if len(filteredItems) < len(flyerItems) {
					zap.S().Debugf("Flyer %d (%s): kept %d of %d items after cosine similarity prefilter", flyer.ID, flyer.Name, len(filteredItems), len(flyerItems))
				}

				matches, err := promptWriter.GetFlyerItemsOnGroceryList(ctx, filteredItems, groceries)
				if err != nil {
					zap.S().Warnf("Error processing flyer %d: %v", flyer.ID, err)
					continue
				}

				if len(matches) > 0 {
					fmt.Fprintf(out, "\n%s - %s:\n", flyer.Merchant, flyer.Name)
					for groceryItem, flyerMatches := range matches {
						fmt.Fprintf(out, "[%s]\n", groceryItem)
						for _, flyerItem := range flyerMatches {
							fmt.Fprintf(out, "  • %s %s ($%s)\n", flyerItem.Brand, flyerItem.Name, flyerItem.Price)
						}
						totalDeals++
					}
				} else {
					zap.S().Debugf("Flyer %d (%s) Had No Matches", flyer.ID, flyer.Name)
				}
			}

			if totalDeals == 0 {
				fmt.Fprintln(out, "No deals found matching your grocery list.")
			} else {
				fmt.Fprintf(out, "Found %d deals total!\n", totalDeals)
			}
			return nil
		},
	}
}

// prefilterFlyerItemsBySimilarity keeps only flyer items whose stored
// brand-plus-name embedding is at least as cosine-similar to any stored
// grocery embedding as comparer requires. Both vectors come from storage
// (no embedder argument). Items with empty embedding text are skipped
// without error; items with non-empty text but a missing stored embedding
// return an error so the user can re-run scrape-flyers.
func prefilterFlyerItemsBySimilarity(
	ctx context.Context,
	comparer *cosinecompare.CosineCompare,
	groceries []storage.GroceryItem,
	items []storage.FlyerItem,
) ([]storage.FlyerItem, error) {
	kept := make([]storage.FlyerItem, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(item.Brand + " " + item.Name)
		if text == "" {
			continue
		}
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("flyer item %d has non-empty text %q but no stored embedding; re-run `scrape-flyers` to backfill", item.ID, text)
		}
		similar := false
		for _, g := range groceries {
			ok, err := comparer.AreCosineSimilar(item.Embedding, g.Embedding)
			if err != nil {
				return nil, fmt.Errorf("find deals: compare grocery %q to flyer item %d: re-add groceries and re-scrape so both vectors come from the current model: %w", g.Name, item.ID, err)
			}
			if ok {
				similar = true
				break
			}
		}
		if similar {
			kept = append(kept, item)
		}
	}
	return kept, nil
}
