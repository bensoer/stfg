package cmd

import (
	"fmt"

	"stfg/internal/promptwriter"
	"stfg/internal/provider"
	"stfg/internal/storage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// FindDealsCmd represents the find-deals command
var FindDealsCmd = &cobra.Command{
	Use:   "find-deals",
	Short: "Find deals in flyers matching your grocery list",
	Long:  `Search through downloaded flyers to find items from your grocery list that are on sale.`,
	Run: func(cmd *cobra.Command, args []string) {

		flyers, err := storage.LoadAllFlyers()
		if err != nil {
			zap.S().Errorf("Error Loading Flyers %v", err)
			return
		}

		if len(flyers) == 0 {
			zap.S().Info("There Are No Flyers Loaded. Have You Scraped First ?")
			return
		}

		// Load grocery list
		groceries, err := storage.LoadGroceries()
		if err != nil {
			zap.S().Errorf("Error loading groceries: %v", err)
			return
		}

		if len(groceries) == 0 {
			fmt.Println("Your grocery list is empty. Please add some items first.")
			return
		}

		// Initialize OpenRouter client
		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			// Fallback to flag if not set in config
			apiKeyFlag, _ := cmd.Flags().GetString("api-key")
			apiKey = apiKeyFlag
		}
		client := provider.NewClient(apiKey)

		// Initialize prompt writer
		promptWriter := promptwriter.NewOpenRouterFreePromptWriter(client)

		zap.S().Infof("Searching for deals on %d grocery items across %d flyers...\n\n", len(groceries), len(flyers))

		// Search through each flyer
		totalDeals := 0
		for _, flyer := range flyers {

			flyerItems, err := storage.LoadFlyerItems(flyer.ID)
			// Load flyer items
			if err != nil {
				zap.S().Warnf("Error loading flyer %d: %v", flyer.ID, err)
				continue
			}

			if len(flyerItems) == 0 {
				continue
			}

			// Find matches using OpenRouter
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
