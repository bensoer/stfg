package bootstrap

import (
	"stfg/internal/commands"
	"stfg/internal/container"

	"github.com/spf13/cobra"
)

func RegisterSubCommands(rootCmd *cobra.Command, container *container.ContainerRegistry) *cobra.Command {

	rootCmd.AddCommand(commands.ScrapeFlyersCmd(container))
	rootCmd.AddCommand(commands.FindDealsCmd(container))

	groceriesCmd := commands.GroceriesCmd(rootCmd, container)
	groceriesCmd.AddCommand(commands.AddGroceryCmd(container))
	groceriesCmd.AddCommand(commands.RemoveGroceryCmd(container))
	groceriesCmd.AddCommand(commands.ListGroceriesCmd(container))

	rootCmd.AddCommand(groceriesCmd)

	return rootCmd
}
