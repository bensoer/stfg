package provider

// GroceryFlyerMatch is one grocery list item matched to one flyer item.
// You are a helpful shopping assistant. Below is a store flyer and a list of groceries the user wants to buy.

// Decide which items from the grocery list appear (or have a close match/equivalent) in the flyer. For each match, report the grocery item, the matching flyer item id, and the matching flyer item name if available.

// Respond ONLY with a JSON array of objects, each with the keys "grocery_item", "flyer_item_id" and "flyer_item_name". If there are no matches, respond with an empty array. If a grocery does not have a match DO NOT create an entry with empty flyer_item_id and flyer_item_name values. Do not include it in the response instead
type GroceryFlyerMatch struct {
	GroceryItem   string `json:"grocery_item"`
	FlyerItemID   int64  `json:"flyer_item_id"`
	FlyerItemName string `json:"flyer_item_name"`
}

// GroceryFlyerMatches is the object the model is asked to return.
type GroceryFlyerMatches struct {
	Matches []GroceryFlyerMatch `json:"matches"`
}
