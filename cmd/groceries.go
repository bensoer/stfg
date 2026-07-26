package cmd

import (
	"github.com/spf13/cobra"
	"stfg/internal/storage"
)

var groceriesCmd = &cobra.Command{
	Use:   "groceries",
	Short: "Grocery list management commands",
	Long:  "Commands to add, remove, and list grocery items",
}

func init() {
	rootCmd.AddCommand(groceriesCmd)

	groceriesCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return storage.EnsureGroceryFileExists()
	}
}