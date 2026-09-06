package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"stfg/internal/models"
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
		embedding, err := models.CreateGroceryEmbedding(item)
		if err != nil {
			zap.S().Error("Failed To Create Grocery Embedding Data", err)
			return
		}

		newGrocery := storage.GroceryItem{
			Name:      item,
			Embedding: embedding,
		}

		if err := store.AddGrocery(cmd.Context(), newGrocery); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error saving groceries: %v\n", err)
			return
		}
		fmt.Printf("\nGrocery list updated: %s added\n", item)
	},
}

func init() {
	groceriesCmd.AddCommand(addGroceryCmd)
}
