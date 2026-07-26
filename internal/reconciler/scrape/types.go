package scrape

type ScrapeReconcilerOptions struct {
	PostalCode           string
	RetailGroupWhiteList []string
}

type ScrapeClient interface {
	GetRetailGroups(postalCode string) ([]RetailGroup, error)
	GetRetailGroupItems(retailGroupId int64) ([]RetailGroupItem, error)
}

type StorageClient interface {
	HasRetailGroup(retailGroup RetailGroup) bool
	HasRetailGroupItem(retailGroupItem RetailGroupItem) (bool, error)

	AddRetailGroup(retailGroup RetailGroup) error
	AddRetailGroupItem(retailGroupItem RetailGroupItem) error

	RemoveRetailGroup(retailGroup RetailGroup) error
	RemoveRetailGroupItem(retailGroupItem RetailGroupItem) error

	GetRetailGroupItems(retailGroupId int64) ([]RetailGroupItem, error)
	GetAllRetailGroups() ([]RetailGroup, error)
}

// A "group" is the generic name for a flyer or a store
// it includes identifiers for the grouping and locations details
type RetailGroup struct {
	ID        int64  `json:"id"`
	ValidFrom string `json:"validFrom,"`
	ValidTo   string `json:"validTo"`

	Name     string `json:"name"`
	Merchant string `json:"merchant"`

	// contains extra mapping relevent for other systems and debugging
	Aux map[string]string `json:"aux"`

	Locations []RetailGroupLocation `json:"location"`
}

type RetailGroupLocation struct {
	ID         int    `json:"id"`
	Address    string `json:"address"`
	City       string `json:"city"`
	Province   string `json:"province"`
	PostalCode string `json:"postalCode"`
}

type RetailGroupItem struct {
	ID            int64 `json:"id"`
	RetailGroupId int64 `json:"retailGroupId"`

	Name  string `json:"name"`
	Brand string `json:"brand,omitempty"`
	Price string `json:"price,omitempty"`

	CutoutImageURL string  `json:"cutout_image_url,omitempty"`
	VideoURL       *string `json:"video_url,omitempty"`
}
