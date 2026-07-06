/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"stfg/internal"
	"stfg/internal/flipp"
	"stfg/internal/storage"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// scrapeFlyersCmd represents the scrapeFlyers command
var scrapeFlyersCmd = &cobra.Command{
	Use:   "scrape-flyers [postal-code]",
	Short: "Parse and scrape flyers from grocers of choice",
	Args:  cobra.ExactArgs(1),
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		today := time.Now()
		postalCode := args[0]

		zap.S().Info("Searching For Valid Deals For ", today.Format("2006-01-02"), " Near Postal Code: ", postalCode)

		client := flipp.NewClient()

		flyers, err := client.GetFlyers(postalCode)
		if err != nil {
			zap.S().Error("Error getting flyers: ", err)
			return
		}

		validFlyers := []string{
			"Superstore",
			"Thrify Foods",
			"Quality Foods",
			"Buy-Low Foods",
			"Country Grocer",
			"No Frills",
			"Pharmasave",
			"Shoppers Drug Mart",
			"Walmart",
			"Nesters Market",
			"Rexall",
		}

		flyerIndex := map[int64]string{}

		for _, f := range flyers.Flyers {
			if !internal.Contains(validFlyers, f.Merchant) {
				// If its not a merchant we care about, skip it
				continue
			}

			availableFrom, err := internal.ParseDate(f.AvailableFrom)
			if err != nil {
				zap.S().Error("Error parsing available_from date: ", err)
				continue
			}

			availableTo, err := internal.ParseDate(f.AvailableTo)
			if err != nil {
				zap.S().Error("Error parsing available_to date: ", err)
				continue
			}

			if availableFrom.After(today) || availableTo.Before(today) {
				// If the flyer is not available today, skip it
				continue
			}

			fmt.Println(f.Merchant, "-", f.Name)

			items, err := client.GetFlyerItems(f.ID)
			if err != nil {
				zap.S().Error("Error getting flyer items: ", err)
				return
			}
			fmt.Println(" items:", len(*items))

			stores, err := client.GetNearbyStores(f.ID, postalCode)
			if err != nil {
				zap.S().Error("Error getting nearby stores: ", err)
				return
			}
			fmt.Println(" stores:", len(*stores))

			flyerFileName := storage.FlyerFileName(f.ID)
			if err := storage.SaveJSON(flyerFileName, items); err != nil {
				zap.S().Error("Error saving flyer: ", err)
				return
			}
			flyerIndex[f.ID] = fmt.Sprintf("%s/%s", f.Merchant, f.Name)

		}

		flyersIndexFile := storage.FlyersIndexFile()
		if err := storage.SaveJSON(flyersIndexFile, flyerIndex); err != nil {
			zap.S().Error("Error saving flyers index: ", err)
			return
		}

	},
}

func init() {
	rootCmd.AddCommand(scrapeFlyersCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// scrapeFlyersCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// scrapeFlyersCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
