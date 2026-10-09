package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"stfg/internal/container"
	"stfg/internal/embedding"
	"stfg/internal/storage"
)

func AddGroceryCmd(container *container.ContainerRegistry) *cobra.Command {

	return &cobra.Command{
		Use:   "add-grocery [item]",
		Short: "Add a new grocery item to the list",
		Long:  "Adds a grocery item to the list with case-insensitive duplicate checking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item := args[0]
			out := cmd.OutOrStdout()

			store, err := container.GetStorage(cmd.Context(), "json")
			if err != nil {
				return fmt.Errorf("Error initializing storage: %w", err)
			}

			provider, err := container.GetProvider(cmd.Context(), "openrouter")
			if err != nil {
				return fmt.Errorf("Error initializing provider: %w", err)
			}

			has, err := store.HasGrocery(cmd.Context(), item)
			if err != nil {
				return fmt.Errorf("Error checking groceries: %w", err)
			}
			if has {
				fmt.Fprintf(out, "Item '%s' already exists\n", item)
				return nil
			}

			zap.S().Info("Getting Embedding Value For Grocery")

			embedder, err := embedding.NewGroceryEmbedder(provider, "openai/text-embedding-3-small")
			if err != nil {
				return fmt.Errorf("Failed To Create Grocery Embedding Data: %w", err)
			}
			vector, err := embedder.CreateGroceryEmbedding(cmd.Context(), item)
			if err != nil {
				return fmt.Errorf("Failed To Create Grocery Embedding Data: %w", err)
			}

			newGrocery := storage.GroceryItem{
				Name:      item,
				Embedding: vector,
			}

			if err := store.AddGrocery(cmd.Context(), newGrocery); err != nil {
				return fmt.Errorf("Error saving groceries: %w", err)
			}
			fmt.Fprintf(out, "\nGrocery list updated: %s added\n", item)
			return nil
		},
	}
}
