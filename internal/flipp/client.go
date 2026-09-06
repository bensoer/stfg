package flipp

import (
	"encoding/json"
	"fmt"

	"github.com/h2non/gentleman"

	"stfg/internal"
	"stfg/internal/reconciler/scrape"
	"stfg/internal/storage"
)

type Client struct {
	base *gentleman.Client
}

func NewClient() *Client {
	return &Client{
		base: gentleman.New(),
	}
}

func (c *Client) GetRetailGroups(postalCode string) ([]storage.Flyer, error) {
	resp, err := c.GetFlyers(postalCode)
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
			continue
		}
		from, err := internal.ParseDate(*validFromStr)
		if err != nil {
			return nil, err
		}

		validToStr := flyer.ValidTo
		if validToStr == nil {
			validToStr = flyer.AvailableTo
		}
		if validToStr == nil {
			continue
		}
		to, err := internal.ParseDate(*validToStr)
		if err != nil {
			return nil, err
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

func (c *Client) GetRetailGroupItems(retailGroupId int64) ([]storage.FlyerItem, error) {
	resp, err := c.GetFlyerItems(retailGroupId)
	if err != nil {
		return nil, err
	}

	rgis := []storage.FlyerItem{}
	for _, flyerItem := range *resp {
		var videoURL string
		if flyerItem.VideoURL != nil {
			videoURL = *flyerItem.VideoURL
		}

		rgis = append(rgis, storage.FlyerItem{
			ID:          flyerItem.ID,
			FlyerID:     retailGroupId,
			Name:        flyerItem.Name,
			Brand:       flyerItem.Brand,
			Price:       flyerItem.Price,
			ImageURL:    flyerItem.CutoutImageURL,
			VideoURL:    videoURL,
			DisplayType: flyerItem.DisplayType,
		})
	}

	return rgis, nil

}

func (c *Client) GetFlyers(postalCode string) (*GetFlyersResponse, error) {
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

var _ scrape.ScrapeClient = (*Client)(nil)
