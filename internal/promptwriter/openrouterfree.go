package promptwriter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"stfg/internal/storage"

	"stfg/internal"

	"go.uber.org/zap"
)

type OpenRouterFreePromptWriter struct {
	providerPrompter ProviderPrompter
	model            string
}

func NewOpenRouterFreePromptWriter(pp ProviderPrompter) *OpenRouterFreePromptWriter {
	return &OpenRouterFreePromptWriter{
		providerPrompter: pp,
		model:            "openrouter/free",
	}
}

func (o *OpenRouterFreePromptWriter) GetFlyerItemsOnGroceryList(ctx context.Context, flyerItems []storage.FlyerItem, groceryList []storage.GroceryItem) (map[string][]storage.FlyerItem, error) {

	minifiedFlyerItems := []map[string]any{}
	for _, flyerItem := range flyerItems {
		minifiedFlyerItems = append(minifiedFlyerItems, map[string]any{
			"id":          flyerItem.ID,
			"brand":       flyerItem.Brand,
			"name":        flyerItem.Name,
			"displayType": flyerItem.DisplayType,
			"imgURL":      flyerItem.ImageURL,
		})
	}

	flyerJSON, err := json.MarshalIndent(minifiedFlyerItems, "", "  ")
	if err != nil {
		return nil, err
	}

	groceryNames := make([]string, len(groceryList))
	for i, g := range groceryList {
		groceryNames[i] = g.Name
	}

	groceryJSON, err := json.MarshalIndent(groceryNames, "", "  ")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(
		`# Task
Review the following flyer and grocery list. Find all items in the flyer that match to items on the grocery list. For each match, create a JSON object in the JSON list. Once complete, respond with the entire JSON list.

# Response Structure

ALWAYS respond in with a JSON list of objects matching this spec:
{
  "matches": [
    {
      "grocery_item": string,
      "flyer_item_id": number,
      "flyer_item_name": string
    }
  ]
}

## Attributes Description:
- "grocery_item" is the grocery item from the grocery list
- "flyer_item_id" is the id value of the flyer item
- "flyer_item_name" is the name of the flyer item

# Rules
- NEVER respond with anything else but the JSON list of JSON objects described in the response structure
- NEVER respond with text before or after the JSON
- NEVER respond with markdown syntax
- NEVER create an entry in the JSON array if there is 0 matching flyer items for a given grocery item
- IF there are no matching flyer items, return an empty JSON array

# Data

--- FLYER ---
%s

--- GROCERY LIST ---
%s`, flyerJSON, groceryJSON)

RetryLoop:
	for range 3 {
		gfms, err := o.providerPrompter.Send(ctx, prompt, o.model)
		if err != nil {
			// Something bizarre happened, we should abort right away
			zap.S().Errorf("Error sending prompt: %v", err)
			return nil, fmt.Errorf("error sending prompt: %w", err)
		}

		zap.S().Debug("Repairs were fine, on to checking if things match")

		// Next check that they all map

		flyerItemsMatchingGroceries := map[string][]storage.FlyerItem{}
		for _, gfm := range gfms {

			matchFound := false
			for _, flyerItem := range flyerItems {
				if flyerItem.ID == gfm.FlyerItemID &&
					flyerItem.Name == gfm.FlyerItemName &&
					internal.Contains(groceryNames, gfm.GroceryItem) {

					// then this item is indeed a match!

					value, ok := flyerItemsMatchingGroceries[gfm.GroceryItem]
					if ok {
						value = append(value, flyerItem)
						flyerItemsMatchingGroceries[gfm.GroceryItem] = value
					} else {
						flyerItemsMatchingGroceries[gfm.GroceryItem] = []storage.FlyerItem{
							flyerItem,
						}
					}

					matchFound = true
					break
				}
			}

			if !matchFound {
				// This means there is a response item that doesn't belong to anything! We got illogical mappings!
				// fix any bizarreness in the response
				// Repairing was not possible, response was invalid. We should try again
				// Parsing the returned object was not possible. Response is invalid. We should try again
				zap.S().Debugf("No Match For: FlyerItemName %s |  FlyerItemId %d | GroceryItem %s", gfm.FlyerItemName, gfm.FlyerItemID, gfm.GroceryItem)
				// fix any bizarreness in the response
				continue RetryLoop
			}

		}

		// If we get this far then everything worked. Return the contents
		return flyerItemsMatchingGroceries, nil

	}

	// If we got here that means the retry loop ran out. We couldn't get anything valuable back from the LLM
	return nil, errors.New("Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer")

}
