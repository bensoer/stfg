package flipp

import (
	"encoding/json"
	"fmt"

	"github.com/h2non/gentleman"
)

type Client struct {
	base *gentleman.Client
}

func NewClient() *Client {
	return &Client{
		base: gentleman.New(),
	}
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
