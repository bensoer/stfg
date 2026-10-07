package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"stfg/internal/embedding"
	"stfg/internal/provider"
	"stfg/internal/storage"
)

var addGroceryCmd = &cobra.Command{
	Use:   "add-grocery [item]",
	Short: "Add a new grocery item to the list",
	Long:  "Adds a grocery item to the list with case-insensitive duplicate checking",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		item := args[0]

		store := getStore(cmd)

		has, err := store.HasGrocery(cmd.Context(), item)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error checking groceries: %v\n", err)
			return
		}
		if has {
			fmt.Printf("Item '%s' already exists\n", item)
			return
		}

		zap.S().Info("Getting Embedding Value For Grocery")

		apiKey := viper.GetString("api_key")
		if apiKey == "" {
			apiKeyFlag, _ := cmd.Flags().GetString("api-key")
			apiKey = apiKeyFlag
		}
		client, err := provider.NewOpenRouterProvider(apiKey)
		if err != nil {
			zap.S().Error("Failed To Create Grocery Embedding Data", err)
			return
		}
		embedder, err := embedding.NewGroceryEmbedder(client, "openai/text-embedding-3-small")
		if err != nil {
			zap.S().Error("Failed To Create Grocery Embedding Data", err)
			return
		}
		vector, err := embedder.CreateGroceryEmbedding(cmd.Context(), item)
		if err != nil {
			zap.S().Error("Failed To Create Grocery Embedding Data", err)
			return
		}

		newGrocery := storage.GroceryItem{
			Name:      item,
			Embedding: vector,
		}

		if err := store.AddGrocery(cmd.Context(), newGrocery); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error saving groceries: %v\n", err)
			return
		}
		fmt.Printf("\nGrocery list updated: %s added\n", item)
	},
}

func init() {
	addGroceryCmd.Flags().StringP("api-key", "a", "", "API key (optional, can be set via config file)")
	viper.BindPFlag("api_key", addGroceryCmd.Flags().Lookup("api-key"))
	groceriesCmd.AddCommand(addGroceryCmd)
}
