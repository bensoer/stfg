package commands

import (
	"fmt"
	"stfg/internal/container"

	"github.com/spf13/cobra"
)

func ListGroceriesCmd(container *container.ContainerRegistry) *cobra.Command {

	return &cobra.Command{
		Use:   "list-groceries",
		Short: "List all grocery items in the list",
		Long:  "Lists all grocery items in the JSON file list",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			store, err := container.GetStorage(cmd.Context(), "json")
			if err != nil {
				return fmt.Errorf("Error initializing storage: %w", err)
			}

			groceries, err := store.ListGroceries(cmd.Context())
			if err != nil {
				return fmt.Errorf("Error loading groceries: %w", err)
			}

			if len(groceries) == 0 {
				fmt.Fprintln(out, "Groceries list is empty")
				return nil
			}

			fmt.Fprintf(out, "Groceries (%d items):\n", len(groceries))
			for i, item := range groceries {
				fmt.Fprintf(out, "  %d. %s\n", i+1, item.Name)
			}
			return nil
		},
	}
}
