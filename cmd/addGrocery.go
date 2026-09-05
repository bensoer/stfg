package cmd

import (
	"fmt"
	"strings"

	"stfg/internal/models"
	"stfg/internal/storage"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var addGroceryCmd = &cobra.Command{
	Use:   "add-grocery [item]",
	Short: "Add a new grocery item to the list",
	Long:  "Adds a grocery item to the list with case-insensitive duplicate checking",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		item := args[0]

		jsonStorage, err := storage.NewJSONFileStorage()
		if err != nil {
			zap.S().Error("Error Loading Storage")
			zap.S().Error(err)
			return
		}

		groceries, err := jsonStorage.GetAllGroceries()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error loading groceries: %v\n", err)
			return
		}

		for _, existing := range groceries {
			if strings.EqualFold(existing.Name, item) {
				fmt.Printf("Item '%s' already exists\n", item)
				return
			}
		}

		zap.S().Info("Getting Embedding Value For Grocery")
		// Create embeddings
		embedding, err := models.CreateGroceryEmbedding(item)
		if err != nil {
			zap.S().Error("Failed To Create Grocery Embedding Data", err)
			return
		}

		newGrocery := storage.GroceryItem{
			Name:      item,
			Embedding: embedding,
		}

		err = jsonStorage.AddGrocery(newGrocery)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error saving groceries: %v\n", err)
			return
		}
		fmt.Printf("\nGrocery list updated: %s added\n", item)
	},
}

func init() {
	groceriesCmd.AddCommand(addGroceryCmd)
}
