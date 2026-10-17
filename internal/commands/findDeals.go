package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"stfg/internal/container"
	"stfg/internal/cosinecompare"
	"stfg/internal/embedding"
	"stfg/internal/promptwriter"
	"stfg/internal/storage"
)

// groceryEmbeddingModel is the model string used to embed flyer items for
// cosine-similarity prefiltering. It must match the model used in
// internal/commands/addGrocery.go so stored grocery embeddings and the
// freshly-embedded flyer vectors live in the same coordinate space.
const groceryEmbeddingModel = "nvidia/nemotron-3-embed-1b:free"

func FindDealsCmd(container *container.ContainerRegistry) *cobra.Command {

	// FindDealsCmd represents the find-deals command
	return &cobra.Command{
		Use:   "find-deals",
		Short: "Find deals in flyers matching your grocery list",
		Long:  `Search through downloaded flyers to find items from your grocery list that are on sale. Each flyer item is embedded from brand and name and dropped unless it is at least 60% cosine-similar to a stored grocery embedding; the remaining items are still matched by the model.`,
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

			// Warn once per grocery whose stored embedding is missing, then
			// build a working list of groceries that already have one. The
			// plan stops the run when no grocery has a usable embedding so
			// the user re-adds them.
			goodGroceries := make([]storage.GroceryItem, 0, len(groceries))
			for _, g := range groceries {
				if len(g.Embedding) == 0 {
					zap.S().Warnf("Grocery %q has no embedding stored; re-add it with `groceries add-grocery` to generate one.", g.Name)
					continue
				}
				goodGroceries = append(goodGroceries, g)
			}
			if len(goodGroceries) == 0 {
				return fmt.Errorf("no grocery has a usable embedding; please re-add your groceries with `groceries add-grocery`")
			}

			embedder, err := embedding.NewGroceryEmbedder(provider, groceryEmbeddingModel)
			if err != nil {
				return fmt.Errorf("find deals: create embedder: %w", err)
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

				filteredItems, err := prefilterFlyerItemsBySimilarity(ctx, embedder, comparer, goodGroceries, flyerItems)
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

// prefilterFlyerItemsBySimilarity keeps only flyer items whose brand-plus-name
// embedding is at least as cosine-similar to any grocery embedding as comparer
// requires. The flyer embedding is a local []float32 and is dropped before
// this function returns. Embedding or compare errors are returned to the
// caller; the caller decides whether to abort or to fall back.
func prefilterFlyerItemsBySimilarity(
	ctx context.Context,
	embedder *embedding.GroceryEmbedder,
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
		flyerEmbedding, err := embedder.CreateGroceryEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("find deals: embed flyer item %d: %w", item.ID, err)
		}
		similar := false
		for _, g := range groceries {
			ok, err := comparer.AreCosineSimilar(flyerEmbedding, g.Embedding)
			if err != nil {
				return nil, fmt.Errorf("find deals: compare grocery %q to flyer item %d: %w", g.Name, item.ID, err)
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
