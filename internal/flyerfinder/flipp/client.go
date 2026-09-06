package flipp

import (
	"encoding/json"
	"fmt"

	"gopkg.in/h2non/gentleman.v2"
	"go.uber.org/zap"

	"stfg/internal"
	"stfg/internal/flyerfinder"
	"stfg/internal/storage"
)

type Client struct {
	base *gentleman.Client
}

func NewFinder(opts ...FinderOption) *Client {
	c := &Client{
		base: gentleman.New(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FinderOption configures a Client.
type FinderOption func(*Client)

// WithBaseClient sets the underlying gentleman.Client (useful for testing with a mock).
func WithBaseClient(b *gentleman.Client) FinderOption {
	return func(c *Client) {
		c.base = b
	}
}

var _ flyerfinder.FlyerFinder = (*Client)(nil)

func (c *Client) GetFlyers(postalCode string) ([]storage.Flyer, error) {
	resp, err := c.GetFlyersResponse(postalCode)
	if err != nil {
		return nil, err
	}

	flyers := []storage.Flyer{}
	for _, flyer := range resp.Flyers {

		storeResp, err := c.GetNearbyStores(flyer.ID, postalCode)
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

		// Resolve the effective valid-from and valid-to times.
		// Prefer ValidFrom/ValidTo with AvailableFrom/AvailableTo
		// as fallback. Skip the flyer if both are nil to avoid
		// a nil-pointer dereference.
		validFromStr := flyer.ValidFrom
		if validFromStr == nil {
			validFromStr = flyer.AvailableFrom
		}
		if validFromStr == nil {
			zap.S().Warnf("Skipping flyer %d (%s): no valid_from or available_from", flyer.ID, flyer.Name)
			continue
		}
		from, err := internal.ParseDate(*validFromStr)
		if err != nil {
			zap.S().Warnf("Skipping flyer %d (%s): cannot parse valid_from %q", flyer.ID, flyer.Name, *validFromStr)
			continue
		}

		validToStr := flyer.ValidTo
		if validToStr == nil {
			validToStr = flyer.AvailableTo
		}
		if validToStr == nil {
			zap.S().Warnf("Skipping flyer %d (%s): no valid_to or available_to", flyer.ID, flyer.Name)
			continue
		}
		to, err := internal.ParseDate(*validToStr)
		if err != nil {
			zap.S().Warnf("Skipping flyer %d (%s): cannot parse valid_to %q", flyer.ID, flyer.Name, *validToStr)
			continue
		}

		flyers = append(flyers, storage.Flyer{
			ID:        flyer.ID,
			ValidFrom: from,
			ValidTo:   to,
			Name:      flyer.Name,
			Merchant:  flyer.Merchant,
			Stores:    stores,
		})
	}

	return flyers, nil
}

func (c *Client) GetFlyerItems(flyerID int64) ([]storage.FlyerItem, error) {
	resp, err := c.GetFlyerItemsResponse(flyerID)
	if err != nil {
		return nil, err
	}

	items := []storage.FlyerItem{}
	for _, flyerItem := range *resp {
		var videoURL string
		if flyerItem.VideoURL != nil {
			videoURL = *flyerItem.VideoURL
		}

		items = append(items, storage.FlyerItem{
			ID:          flyerItem.ID,
			FlyerID:     flyerID,
			Name:        flyerItem.Name,
			Brand:       flyerItem.Brand,
			Price:       flyerItem.Price,
			ImageURL:    flyerItem.CutoutImageURL,
			VideoURL:    videoURL,
			DisplayType: flyerItem.DisplayType,
		})
	}

	return items, nil
}

func (c *Client) GetFlyersResponse(postalCode string) (*GetFlyersResponse, error) {
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

func (c *Client) GetFlyerItemsResponse(flyerID int64) (*GetFlyerItemsResponse, error) {
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

// FindFlyers delegates to GetFlyers for FlyerFinder interface compliance.
func (c *Client) FindFlyers(postalCode string) ([]storage.Flyer, error) {
	return c.GetFlyers(postalCode)
}

// FindFlyerItems delegates to GetFlyerItems for FlyerFinder interface compliance.
func (c *Client) FindFlyerItems(flyerID int64) ([]storage.FlyerItem, error) {
	return c.GetFlyerItems(flyerID)
}
