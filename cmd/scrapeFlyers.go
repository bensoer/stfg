/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"stfg/internal/flipp"
	"stfg/internal/reconciler/scrape"
	"stfg/internal/storage"
	"stfg/internal/storage/json"
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
		store, err := json.NewJSON(cmd.Context())
		if err != nil {
			zap.S().Error("Failed To Setup Storage System")
			zap.S().Error(err)
			return
		}

		var s storage.Storage = store

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

		err = scrape.Reconcile(cmd.Context(), client, s, scrape.ScrapeReconcilerOptions{
			PostalCode:           postalCode,
			RetailGroupWhiteList: validFlyers,
		})

		if err != nil {
			zap.S().Error("Error Scraping Flyers")
			zap.S().Error(err)
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
