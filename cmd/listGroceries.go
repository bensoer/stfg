package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listGroceriesCmd = &cobra.Command{
	Use:   "list-groceries",
	Short: "List all grocery items in the list",
	Long:  "Lists all grocery items in the JSON file list",
	Run: func(cmd *cobra.Command, args []string) {
		store := getStore(cmd)

		groceries, err := store.ListGroceries(cmd.Context())
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error loading groceries: %v\n", err)
			return
		}

		if len(groceries) == 0 {
			fmt.Println("Groceries list is empty")
			return
		}

		fmt.Printf("Groceries (%d items):\n", len(groceries))
		for i, item := range groceries {
			fmt.Printf("  %d. %s\n", i+1, item.Name)
		}
	},
}

func init() {
	groceriesCmd.AddCommand(listGroceriesCmd)
}
