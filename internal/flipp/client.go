package flipp

import (
	"encoding/json"
	"fmt"

	"github.com/h2non/gentleman"

	"stfg/internal/reconciler/scrape"
)

type Client struct {
	base *gentleman.Client
}

func NewClient() *Client {
	return &Client{
		base: gentleman.New(),
	}
}

func (c *Client) GetRetailGroups(postalCode string) ([]scrape.RetailGroup, error) {
	resp, err := c.GetFlyers(postalCode)
	if err != nil {
		return nil, err
	}

	rg := []scrape.RetailGroup{}
	for _, flyer := range resp.Flyers {

		storeResp, err := c.GetNearbyStores(flyer.ID, postalCode)
		if err != nil {
			return nil, err
		}

		rgl := []scrape.RetailGroupLocation{}
		for _, store := range *storeResp {
			rgl = append(rgl, scrape.RetailGroupLocation{
				ID:         store.ID,
				Address:    store.Address,
				City:       store.City,
				PostalCode: store.PostalCode,
				Province:   store.Province,
			})
		}

		validFrom := flyer.ValidFrom
		if validFrom == nil {
			validFrom = flyer.AvailableFrom
		}

		validTo := flyer.ValidTo
		if validTo == nil {
			validTo = flyer.AvailableTo
		}

		rg = append(rg, scrape.RetailGroup{
			ID:        flyer.ID,
			ValidFrom: *validFrom,
			ValidTo:   *validTo,
			Name:      flyer.Name,
			Merchant:  flyer.Merchant,

			Locations: rgl,
		})
	}

	return rg, nil
}

func (c *Client) GetRetailGroupItems(retailGroupId int64) ([]scrape.RetailGroupItem, error) {
	resp, err := c.GetFlyerItems(retailGroupId)
	if err != nil {
		return nil, err
	}

	rgis := []scrape.RetailGroupItem{}
	for _, flyerItem := range *resp {
		rgis = append(rgis, scrape.RetailGroupItem{
			ID:             flyerItem.ID,
			RetailGroupId:  retailGroupId,
			Name:           flyerItem.Name,
			Brand:          flyerItem.Brand,
			Price:          flyerItem.Price,
			CutoutImageURL: flyerItem.CutoutImageURL,
			VideoURL:       flyerItem.VideoURL,
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
