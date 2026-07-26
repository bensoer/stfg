package cmd

import (
	"fmt"
	"strings"

	"stfg/internal/storage"

	"github.com/spf13/cobra"
)

var addGroceryCmd = &cobra.Command{
	Use:   "add-grocery [item]",
	Short: "Add a new grocery item to the list",
	Long:  "Adds a grocery item to the list with case-insensitive duplicate checking",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		item := args[0]

		groceries, err := storage.LoadGroceries()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error loading groceries: %v\n", err)
			return
		}

		for _, existing := range groceries {
			if strings.EqualFold(existing, item) {
				fmt.Printf("Item '%s' already exists\n", item)
				return
			}
		}

		groceries = append(groceries, item)

		err = storage.SaveGroceries(groceries)
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
