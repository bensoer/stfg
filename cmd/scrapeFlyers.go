/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"stfg/internal/flyerfinder"
	"stfg/internal/flyerfinder/flipp"
	"stfg/internal/reconciler/scrape"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// scrapeFlyersCmd represents the scrapeFlyers command
var scrapeFlyersCmd = &cobra.Command{
	Use:   "scrape-flyers [postal-code]",
	Short: "Parse and scrape flyers from grocers of choice",
	Args:  cobra.ExactArgs(1),
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		postalCode := args[0]

		finder := flipp.NewFinder()
		var f flyerfinder.FlyerFinder = finder
		store := getStore(cmd)

		whitelist := viper.GetStringSlice("fly_finder.whitelist")
		if len(whitelist) == 0 {
			zap.S().Warn("fly_finder.whitelist is empty; no flyers will match")
		}

		err := scrape.Reconcile(cmd.Context(), f, store, scrape.ScrapeReconcilerOptions{
			PostalCode:           postalCode,
			RetailGroupWhiteList: whitelist,
		})

		if err != nil {
			zap.S().Error("Error Scraping Flyers", err)
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
