package cmd

import (
	"fmt"

	"stfg/internal/storage"

	"github.com/spf13/cobra"
)

var listGroceriesCmd = &cobra.Command{
	Use:   "list-groceries",
	Short: "List all grocery items in the list",
	Long:  "Lists all grocery items in the JSON file list",
	Run: func(cmd *cobra.Command, args []string) {
		groceries, err := storage.LoadGroceries()
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
			fmt.Printf("  %d. %s\n", i+1, item)
		}
	},
}

func init() {
	groceriesCmd.AddCommand(listGroceriesCmd)
}
