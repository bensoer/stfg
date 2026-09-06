package storage

import "time"

// GroceryItem is one item on the user's shopping list with its embedding.
type GroceryItem struct {
	Name      string    `json:"name"`
	Embedding []float32 `json:"embedding,omitempty"`
}

// Flyer is one retailer's flyer / retail group.
type Flyer struct {
	ID        int64     `json:"id"`
	ValidFrom time.Time `json:"valid_from"`
	ValidTo   time.Time `json:"valid_to"`
	Name      string    `json:"name"`
	Merchant  string    `json:"merchant"`
	Stores    []Store   `json:"stores"`
}

// FlyerItem is one item inside a flyer.
type FlyerItem struct {
	ID      int64  `json:"id"`
	FlyerID int64  `json:"flyer_id"`
	Name    string `json:"name"`
	Brand   string `json:"brand,omitempty"`
	Price   string `json:"price,omitempty"`

	ImageURL string `json:"image_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`

	// DisplayType is preserved from the Flipp API for the prompt-writer, which
	// currently sends it to the OpenRouter LLM as part of the flyer-item
	// payload. It is not read anywhere else in the codebase.
	DisplayType int `json:"display_type"`
}

// Store is one physical store carrying a flyer.
type Store struct {
	ID         int    `json:"id"`
	Address    string `json:"address"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postal_code"`
}
