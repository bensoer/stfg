package promptwriter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"stfg/internal"
	"stfg/internal/provider"
	"stfg/internal/storage"

	"go.uber.org/zap"
)

// PromptWriter builds the grocery/flyer match prompt, sends it through any
// provider.Provider, and maps the returned matches back onto real
// storage.FlyerItem rows.
type PromptWriter struct {
	provider provider.Provider
	model    string
}

// NewPromptWriter constructs a PromptWriter bound to the given provider and
// model. It returns an error if the provider is nil or the model is empty.
func NewPromptWriter(p provider.Provider, model string) (*PromptWriter, error) {
	if p == nil {
		return nil, fmt.Errorf("prompt writer: provider is nil")
	}
	if model == "" {
		return nil, fmt.Errorf("prompt writer: model is empty")
	}
	return &PromptWriter{provider: p, model: model}, nil
}

// GetFlyerItemsOnGroceryList builds the match prompt and returns a map of
// matched grocery items to flyer items, retrying the provider up to three
// times when a returned match cannot be reconciled with the inputs.
func (p *PromptWriter) GetFlyerItemsOnGroceryList(ctx context.Context, flyerItems []storage.FlyerItem, groceryList []storage.GroceryItem) (map[string][]storage.FlyerItem, error) {

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
Review the following flyer and grocery list. Find all items in the flyer that match to items on the grocery list. For each match, create a JSON object in the matches list. Once complete, respond with the entire JSON object.

# Response Structure

ALWAYS respond in with a JSON object matching this spec:
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
- NEVER respond with anything else but the JSON object described in the response structure
- NEVER respond with text before or after the JSON
- NEVER respond with markdown syntax
- NEVER create an entry in the matches list if there is 0 matching flyer items for a given grocery item
- IF there are no matching flyer items, return an empty matches array

# Data

--- FLYER ---
%s

--- GROCERY LIST ---
%s`, flyerJSON, groceryJSON)

RetryLoop:
	for range 3 {
		// Sends a prompt to a model and returns response as a map[string]any
		gfms, err := p.provider.Send(ctx, prompt, p.model)
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
				zap.S().Debugf("No Match For: FlyerItemName %s |  FlyerItemId %d | GroceryItem %s", gfm.FlyerItemName, gfm.FlyerItemID, gfm.GroceryItem)
				continue RetryLoop
			}

		}

		// If we get this far then everything worked. Return the contents
		return flyerItemsMatchingGroceries, nil

	}

	// If we got here that means the retry loop ran out. We couldn't get anything valuable back from the LLM
	return nil, errors.New("Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer")

}
