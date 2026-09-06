package flipp

type FlyerDetail struct {
	FlyerID int
	Items   []FlyerItem
	Stores  []RetailGroupLocation
}

type GetStoresNearByResponse []RetailGroupLocation

type RetailGroupLocation struct {
	ID         int    `json:"id"`
	Address    string `json:"address"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postal_code"`
}

type GetFlyersResponse struct {
	Flyers []Flyer `json:"flyers"`
}

type GetFlyerItemsResponse []FlyerItem

type Flyer struct {
	ID          int64 `json:"id"`
	FlyerRunID  int   `json:"flyer_run_id"`
	FlyerTypeID int   `json:"flyer_type_id"`

	ValidFrom     *string `json:"valid_from,omitempty"`
	ValidTo       *string `json:"valid_to,omitempty"`
	AvailableFrom *string `json:"available_from,omitempty"`
	AvailableTo   *string `json:"available_to,omitempty"`

	Premium bool `json:"premium"`

	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Priority int     `json:"priority"`

	AnalyticsPayload string `json:"analytics_payload"`

	Name       string `json:"name"`
	Merchant   string `json:"merchant"`
	MerchantID int    `json:"merchant_id"`

	Path string `json:"path"`

	Categories  []string  `json:"categories"`
	Resolutions []float64 `json:"resolutions"`

	BudgetID *int `json:"budget_id"` // nullable

	MerchantLogo        string `json:"merchant_logo"`
	PremiumThumbnailURL string `json:"premium_thumbnail_url"`
	MobileThumbnailURL  string `json:"mobile_thumbnail_url"`
	ThumbnailURL        string `json:"thumbnail_url"`
}

type FlyerItem struct {
	ID      int64 `json:"id"`
	FlyerID int64 `json:"flyer_id"`

	Name  string `json:"name"`
	Brand string `json:"brand,omitempty"`

	DisplayType int `json:"display_type"`

	Price string `json:"price,omitempty"`

	CutoutImageURL string  `json:"cutout_image_url"`
	VideoURL       *string `json:"video_url,omitempty"`

	ValidFrom   string `json:"valid_from"`
	ValidTo     string `json:"valid_to"`
	AvailableTo string `json:"available_to"`

	// Spatial coordinates inside flyer page (useful later for OCR / page mapping)
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`

	PageDestination any `json:"page_destination"`
}
