package cmd

import (
	"github.com/spf13/cobra"
)

var groceriesCmd = &cobra.Command{
	Use:   "groceries",
	Short: "Grocery list management commands",
	Long:  "Commands to add, remove, and list grocery items",
}

func init() {
	rootCmd.AddCommand(groceriesCmd)

	groceriesCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if rootCmd.PersistentPreRunE != nil {
			return rootCmd.PersistentPreRunE(cmd, args)
		}
		return nil
	}
}