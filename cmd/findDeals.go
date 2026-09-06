package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"stfg/internal/promptwriter"
	"stfg/internal/provider"
)

// FindDealsCmd represents the find-deals command
var FindDealsCmd = &cobra.Command{
	Use:   "find-deals",
	Short: "Find deals in flyers matching your grocery list",
	Long:  `Search through downloaded flyers to find items from your grocery list that are on sale.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := getStore(cmd)

		flyers, err := store.ListFlyers(cmd.Context())
		if err != nil {
			zap.S().Errorf("Error Loading Flyers %v", err)
			return
		}

		if len(flyers) == 0 {
			zap.S().Info("There Are No Flyers Loaded. Have You Scraped First ?")
			return
		}

		groceries, err := store.ListGroceries(cmd.Context())
		if err != nil {
			zap.S().Errorf("Error loading groceries: %v", err)
			return
		}

		if len(groceries) == 0 {
			fmt.Println("Your grocery list is empty. Please add some items first.")
			return
		}

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKeyFlag, _ := cmd.Flags().GetString("api-key")
			apiKey = apiKeyFlag
		}
		client := provider.NewClient(apiKey)
		promptWriter := promptwriter.NewOpenRouterFreePromptWriter(client)

		zap.S().Infof("Searching for deals on %d grocery items across %d flyers...\n\n", len(groceries), len(flyers))

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

			matches, err := promptWriter.GetFlyerItemsOnGroceryList(flyerItems, groceries)
			if err != nil {
				zap.S().Warnf("Error processing flyer %d: %v", flyer.ID, err)
				continue
			}

			if len(matches) > 0 {
				fmt.Printf("\n%s - %s:\n", flyer.Merchant, flyer.Name)
				for groceryItem, flyerMatches := range matches {
					fmt.Printf("[%s]\n", groceryItem)
					for _, flyerItem := range flyerMatches {
						fmt.Printf("  • %s %s ($%s)\n", flyerItem.Brand, flyerItem.Name, flyerItem.Price)
					}
					totalDeals++
				}
			} else {
				zap.S().Debugf("Flyer %d (%s) Had No Matches", flyer.ID, flyer.Name)
			}
		}

		if totalDeals == 0 {
			fmt.Println("No deals found matching your grocery list.")
		} else {
			fmt.Printf("Found %d deals total!\n", totalDeals)
		}
	},
}

func init() {
	FindDealsCmd.Flags().StringP("api-key", "a", "", "API key (optional, can be set via config file)")
	viper.BindPFlag("api_key", FindDealsCmd.Flags().Lookup("api-key"))
	rootCmd.AddCommand(FindDealsCmd)
}
