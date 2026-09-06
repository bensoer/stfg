package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"stfg/internal/storage"
)

var removeGroceryCmd = &cobra.Command{
	Use:   "remove-grocery [item]",
	Short: "Remove a grocery item from the list",
	Long:  "Removes a grocery item from the list with case-insensitive matching",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		item := args[0]

		store := getStore(cmd)

		err := store.RemoveGrocery(cmd.Context(), item)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				fmt.Printf("Item '%s' not found\n", item)
				return
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Error removing groceries: %v\n", err)
			return
		}
		fmt.Printf("\nGrocery list updated: %s removed\n", item)
	},
}

func init() {
	groceriesCmd.AddCommand(removeGroceryCmd)
}
