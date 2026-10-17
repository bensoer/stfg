/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package commands

import (
	"fmt"
	"stfg/internal/container"
	"stfg/internal/embedding"
	"stfg/internal/flyerfinder"
	"stfg/internal/flyerfinder/flipp"
	"stfg/internal/reconciler/scrape"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func ScrapeFlyersCmd(container *container.ContainerRegistry) *cobra.Command {

	// scrapeFlyersCmd represents the scrapeFlyers command
	return &cobra.Command{
		Use:   "scrape-flyers [postal-code]",
		Short: "Parse and scrape flyers from grocers of choice",
		Args:  cobra.ExactArgs(1),
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			postalCode := args[0]

			finder := flipp.NewFinder(nil)
			var f flyerfinder.FlyerFinder = finder
			store, err := container.GetStorage(cmd.Context(), "json")
			if err != nil {
				return fmt.Errorf("Error initializing storage: %w", err)
			}

			provider, err := container.GetProvider(cmd.Context(), "openrouter")
			if err != nil {
				return fmt.Errorf("Error initializing provider: %w", err)
			}

			// The reconciler embeds each new or changed flyer item at scrape
			// time so find-deals can compare stored vectors without
			// re-embedding. The model string is shared with add-grocery so
			// both sides live in the same coordinate space.
			embedder, err := embedding.NewGroceryEmbedder(provider, groceryEmbeddingModel)
			if err != nil {
				return fmt.Errorf("Error creating grocery embedder: %w", err)
			}

			whitelist := viper.GetStringSlice("fly_finder.whitelist")
			if len(whitelist) == 0 {
				zap.S().Warn("fly_finder.whitelist is empty; no flyers will match")
			}

			err = scrape.Reconcile(cmd.Context(), f, store, scrape.ScrapeReconcilerOptions{
				PostalCode:           postalCode,
				RetailGroupWhiteList: whitelist,
				Embedder:             embedder,
			})
			if err != nil {
				return fmt.Errorf("Error Scraping Flyers: %w", err)
			}
			return nil
		},
	}
}
