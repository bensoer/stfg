package commands

import (
	"stfg/internal/container"

	"github.com/spf13/cobra"
)

func GroceriesCmd(rootCmd *cobra.Command, container *container.ContainerRegistry) *cobra.Command {

	var groceriesCmd = &cobra.Command{
		Use:   "groceries",
		Short: "Grocery list management commands",
		Long:  "Commands to add, remove, and list grocery items",
	}

	groceriesCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if rootCmd.PersistentPreRunE != nil {
			return rootCmd.PersistentPreRunE(cmd, args)
		}
		return nil
	}

	return groceriesCmd
}
