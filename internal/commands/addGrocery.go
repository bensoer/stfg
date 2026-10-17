package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"stfg/internal/container"
	"stfg/internal/embedding"
	"stfg/internal/storage"
)

// groceryEmbeddingModel is the OpenRouter model string used by both
// add-grocery (to embed the user's grocery names) and scrape-flyers (to embed
// the scraped flyer items). Keeping it in one place means stored grocery
// embeddings and stored flyer embeddings live in the same coordinate space,
// so find-deals can compare them with the cosine prefilter without
// re-embedding either side at runtime.
const groceryEmbeddingModel = "nvidia/nemotron-3-embed-1b:free"

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

			embedder, err := embedding.NewGroceryEmbedder(provider, groceryEmbeddingModel)
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
