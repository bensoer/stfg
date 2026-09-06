package flipp

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"gopkg.in/h2non/gentleman.v2"

	"stfg/internal/flyerfinder"
	"stfg/internal/storage"
)

type Client struct {
	base *gentleman.Client
}

// FinderOption configures a Client.
type FinderOption func(*Client)

// NewFinder returns a new Client finder.
// Options can be used to customize the client (e.g., for testing).
func NewFinder(opts ...FinderOption) *Client {
	c := &Client{
		base: gentleman.New(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithBaseClient sets the underlying gentleman.Client (useful for testing with a mock).
func WithBaseClient(b *gentleman.Client) FinderOption {
	return func(c *Client) {
		c.base = b
	}
}

var _ flyerfinder.FlyerFinder = (*Client)(nil)

func (c *Client) FindFlyers(postalCode string) ([]storage.Flyer, error) {
	resp, err := c.GetFlyers(postalCode)
	if err != nil {
		return nil, err
	}

	flyers := []storage.Flyer{}
	for _, flippFlyer := range resp.Flyers {

		storeResp, err := c.GetNearbyStores(flippFlyer.ID, postalCode)
		if err != nil {
			return nil, err
		}

		stores := []storage.Store{}
		for _, store := range *storeResp {
			stores = append(stores, storage.Store{
				ID:         store.ID,
				Address:    store.Address,
				City:       store.City,
				PostalCode: store.PostalCode,
				Province:   store.Province,
			})
		}

		validFrom := flippFlyer.ValidFrom
		if validFrom == nil {
			validFrom = flippFlyer.AvailableFrom
		}
		if validFrom == nil {
			zap.S().Warnf("Skipping flyer %d (%s): no valid_from or available_from", flippFlyer.ID, flippFlyer.Name)
			continue
		}

		validTo := flippFlyer.ValidTo
		if validTo == nil {
			validTo = flippFlyer.AvailableTo
		}
		if validTo == nil {
			zap.S().Warnf("Skipping flyer %d (%s): no valid_to or available_to", flippFlyer.ID, flippFlyer.Name)
			continue
		}

		flyers = append(flyers, storage.Flyer{
			ID:        flippFlyer.ID,
			ValidFrom: *validFrom,
			ValidTo:   *validTo,
			Name:      flippFlyer.Name,
			Merchant:  flippFlyer.Merchant,
			Stores:    stores,
		})
	}

	return flyers, nil
}

func (c *Client) FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error) {
	resp, err := c.GetFlyerItems(flyerID)
	if err != nil {
		return nil, err
	}

	items := []storage.FlyerItem{}
	for _, flippItem := range *resp {
		var videoURL *string
		if flippItem.VideoURL != nil {
			v := *flippItem.VideoURL
			videoURL = &v
		}

		items = append(items, storage.FlyerItem{
			ID:             flippItem.ID,
			FlyerID:        flyerID,
			Name:           flippItem.Name,
			Brand:          flippItem.Brand,
			DisplayType:    flippItem.DisplayType,
			Price:          flippItem.Price,
			CutoutImageURL: flippItem.CutoutImageURL,
			VideoURL:       videoURL,
		})
	}

	return items, nil
}

func (c *Client) GetFlyers(postalCode string) (*GetFlyersResponse, error) {
	// unchanged - keep existing implementation
	sid := generateSID()
	req := c.base.Request()
	req.URL("https://dam.flippenterprise.net/api/flipp/data")
	req.SetQuery("locale", "en")
	req.SetQuery("postal_code", postalCode)
	req.SetQuery("sid", sid)
	res, err := req.Send()
	if err != nil {
		return nil, err
	}
	if !res.Ok {
		return nil, fmt.Errorf("bad response: %d", res.StatusCode)
	}
	body := res.Bytes()
	var parsed GetFlyersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (c *Client) GetFlyerItems(flyerID int64) (*GetFlyerItemsResponse, error) {
	// unchanged - keep existing implementation
	req := c.base.Request()
	url := fmt.Sprintf(
		"https://dam.flippenterprise.net/api/flipp/flyers/%d/flyer_items",
		flyerID,
	)
	req.URL(url)
	req.SetQuery("locale", "en")
	res, err := req.Send()
	if err != nil {
		return nil, err
	}
	if !res.Ok {
		return nil, fmt.Errorf("bad response: %d", res.StatusCode)
	}
	var parsed GetFlyerItemsResponse
	if err := json.Unmarshal(res.Bytes(), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (c *Client) GetNearbyStores(flyerID int64, postalCode string) (*GetStoresNearByResponse, error) {
	// unchanged - keep existing implementation
	req := c.base.Request()
	url := fmt.Sprintf(
		"https://dam.flippenterprise.net/api/flipp/flyers/%d/stores/nearby",
		flyerID,
	)
	req.URL(url)
	req.SetQuery("locale", "en")
	req.SetQuery("postal_code", postalCode)
	res, err := req.Send()
	if err != nil {
		return nil, err
	}
	if !res.Ok {
		return nil, fmt.Errorf("bad response: %d", res.StatusCode)
	}
	var parsed GetStoresNearByResponse
	if err := json.Unmarshal(res.Bytes(), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
