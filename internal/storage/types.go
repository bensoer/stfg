package storage

import "errors"

var (
	RetailGroupNotFound = errors.New("retail group not found")
)

type Store struct {
	ID         int    `json:"id"`
	Address    string `json:"address"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postal_code"`
}

type Flyer struct {
	ID          int64 `json:"id"`
	FlyerRunID  int   `json:"flyer_run_id"`
	FlyerTypeID int   `json:"flyer_type_id"`

	ValidFrom     string `json:"valid_from"`
	ValidTo       string `json:"valid_to"`
	AvailableFrom string `json:"available_from"`
	AvailableTo   string `json:"available_to"`

	Name       string  `json:"name"`
	Merchant   string  `json:"merchant"`
	MerchantID int     `json:"merchant_id"`
	Stores     []Store `json:"stores"`
}

type FlyerItem struct {
	ID      int64 `json:"id"`
	FlyerID int64 `json:"flyer_id"`

	Name  string `json:"name"`
	Brand string `json:"brand,omitempty"`

	DisplayType int `json:"display_type"`

	Price string `json:"price,omitempty"`

	CutoutImageURL string  `json:"cutout_image_url,omitempty"`
	VideoURL       *string `json:"video_url,omitempty"`
}

type GroceryItem struct {
	Name      string    `json:"name"`
	Embedding []float32 `json:"embedding"`
}
