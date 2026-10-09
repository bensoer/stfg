package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"stfg/internal/container"
	"stfg/internal/promptwriter"
)

func FindDealsCmd(container *container.ContainerRegistry) *cobra.Command {

	// FindDealsCmd represents the find-deals command
	return &cobra.Command{
		Use:   "find-deals",
		Short: "Find deals in flyers matching your grocery list",
		Long:  `Search through downloaded flyers to find items from your grocery list that are on sale.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			store, err := container.GetStorage(cmd.Context(), "json")
			if err != nil {
				return fmt.Errorf("Error initializing storage: %w", err)
			}

			provider, err := container.GetProvider(cmd.Context(), "openrouter")
			if err != nil {
				return fmt.Errorf("Error initializing provider: %w", err)
			}

			flyers, err := store.ListFlyers(cmd.Context())
			if err != nil {
				return fmt.Errorf("Error Loading Flyers: %w", err)
			}

			if len(flyers) == 0 {
				fmt.Fprintln(out, "There Are No Flyers Loaded. Have You Scraped First ?")
				return nil
			}

			groceries, err := store.ListGroceries(cmd.Context())
			if err != nil {
				return fmt.Errorf("Error loading groceries: %w", err)
			}

			if len(groceries) == 0 {
				fmt.Fprintln(out, "Your grocery list is empty. Please add some items first.")
				return nil
			}

			apiKey := viper.GetString("api_key")
			if apiKey == "" {
				apiKeyFlag, _ := cmd.Flags().GetString("api-key")
				apiKey = apiKeyFlag
			}

			promptWriter, err := promptwriter.NewPromptWriter(provider, "openrouter/free")
			if err != nil {
				return fmt.Errorf("Error creating prompt writer: %w", err)
			}

			zap.S().Infof("Searching for deals on %d grocery items across %d flyers...", len(groceries), len(flyers))

			totalDeals := 0
			for _, flyer := range flyers {
				flyerItems, err := store.ListFlyerItems(cmd.Context(), flyer.ID)
				if err != nil {
					zap.S().Warnf("Error loading flyer %d: %v", flyer.ID, err)
					continue
				}

				if len(flyerItems) == 0 {
					continue
				}

				matches, err := promptWriter.GetFlyerItemsOnGroceryList(cmd.Context(), flyerItems, groceries)
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
