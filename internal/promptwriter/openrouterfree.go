package promptwriter

import (
	"encoding/json"
	"errors"
	"fmt"
	"stfg/internal/storage"

	"stfg/internal"

	"github.com/kaptinlin/jsonrepair"
	"go.uber.org/zap"
)

// You are a helpful shopping assistant. Below is a store flyer and a list of groceries the user wants to buy.

// Decide which items from the grocery list appear (or have a close match/equivalent) in the flyer. For each match, report the grocery item, the matching flyer item id, and the matching flyer item name if available.

// Respond ONLY with a JSON array of objects, each with the keys "grocery_item", "flyer_item_id" and "flyer_item_name". If there are no matches, respond with an empty array. If a grocery does not have a match DO NOT create an entry with empty flyer_item_id and flyer_item_name values. Do not include it in the response instead

type GroceryFlyerMatch struct {
	GroceryItem   string `json:"grocery_item"`
	FlyerItemId   int64  `json:"flyer_item_id"`
	FlyerItemName string `json:"flyer_item_name"`
}

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

func (o *OpenRouterFreePromptWriter) GetFlyerItemsOnGroceryList(flyerItems []storage.FlyerItem, groceryList []string) (map[string][]storage.FlyerItem, error) {

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

	groceryJSON, err := json.MarshalIndent(groceryList, "", "  ")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(
		`# Task
Review the following flyer and grocery list. Find all items in the flyer that match to items on the grocery list. For each match, create a JSON object in the JSON list. Once complete, respond with the entire JSON list.

# Response Structure

ALWAYS respond in with a JSON list of objects matching this spec:
[
  {
    "grocery_item": string,
	"flyer_item_id": number,
	"flyer_item_name": string
  }
]

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
		response, err := o.providerPrompter.Send(prompt, o.model)
		if err != nil {
			// Something bizarre happened, we should abort right away
			zap.S().Errorf("Error sending prompt: %v", err)
			return nil, fmt.Errorf("error sending prompt: %w", err)
		}

		zap.S().Debugf("Raw Response >>>%s<<<\n", *response)

		repairedJSON, err := jsonrepair.Repair(*response) // fix any bizarreness in the response
		zap.S().Debugf("Raw Repaired JSON >>>%s<<<\n", repairedJSON)
		if err != nil {
			zap.S().Debugf("Repair Failed. Can't do anything with data: %v", err)
			// Repairing was not possible, response was invalid. We should try again
			continue
		}

		zap.S().Debug("Repairs were fine, on to checking if things match")

		var gfms []GroceryFlyerMatch
		err = json.Unmarshal([]byte(repairedJSON), &gfms)
		if err != nil {
			// Parsing the returned object was not possible. Response is invalid. We should try again
			continue
		}

		// Next check that they all map

		flyerItemsMatchingGroceries := map[string][]storage.FlyerItem{}
		for _, gfm := range gfms {

			matchFound := false
			for _, flyerItem := range flyerItems {
				if flyerItem.ID == gfm.FlyerItemId &&
					flyerItem.Name == gfm.FlyerItemName &&
					internal.Contains(groceryList, gfm.GroceryItem) {

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
				zap.S().Debugf("No Match For: FlyerItemName %s |  FlyerItemId %d | GroceryItem %s", gfm.FlyerItemName, gfm.FlyerItemId, gfm.GroceryItem)
				continue RetryLoop
			}

		}

		// If we get this far then everything worked. Return the contents
		return flyerItemsMatchingGroceries, nil

	}

	// If we got here that means the retry loop ran out. We couldn't get anything valuable back from the LLM
	return nil, errors.New("Failed To Get Valid Response From LLM. Unable To Verify Grocery Items Are On Flyer")

}
