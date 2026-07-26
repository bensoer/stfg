package cmd

import (
	"fmt"
	"strings"

	"stfg/internal/storage"

	"github.com/spf13/cobra"
)

var removeGroceryCmd = &cobra.Command{
	Use:   "remove-grocery [item]",
	Short: "Remove a grocery item from the list",
	Long:  "Removes a grocery item from the list with case-insensitive matching",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		item := args[0]

		groceries, err := storage.LoadGroceries()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error loading groceries: %v\n", err)
			return
		}

		found := false
		for i, existing := range groceries {
			if strings.EqualFold(existing, item) {
				groceries = append(groceries[:i], groceries[i+1:]...)
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("Item '%s' not found\n", item)
			return
		}

		err = storage.SaveGroceries(groceries)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error saving groceries: %v\n", err)
			return
		}
		fmt.Printf("\nGrocery list updated: %s removed\n", item)
	},
}

func init() {
	groceriesCmd.AddCommand(removeGroceryCmd)
}
