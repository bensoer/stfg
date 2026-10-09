package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"stfg/internal/container"
	"stfg/internal/storage"
)

func RemoveGroceryCmd(container *container.ContainerRegistry) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-grocery [item]",
		Short: "Remove a grocery item from the list",
		Long:  "Removes a grocery item from the list with case-insensitive matching",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			item := args[0]
			out := cmd.OutOrStdout()

			store, err := container.GetStorage(cmd.Context(), "json")
			if err != nil {
				return fmt.Errorf("Error initializing storage: %w", err)
			}

			err = store.RemoveGrocery(cmd.Context(), item)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					fmt.Fprintf(out, "Item '%s' not found\n", item)
					return nil
				}
				return fmt.Errorf("Error removing groceries: %w", err)
			}
			fmt.Fprintf(out, "\nGrocery list updated: %s removed\n", item)
			return nil
		},
	}
}
