package flipp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gopkg.in/h2non/gentleman.v2"
	gentlemanv2Context "gopkg.in/h2non/gentleman.v2/context"
)

// ptrString returns a pointer to the string passed in.
func ptrString(s string) *string {
	return &s
}

// TestNewFinder_ReturnsClient tests that NewFinder returns a *Client.
func TestNewFinder_ReturnsClient(t *testing.T) {
	c := NewFinder()
	if c == nil {
		t.Error("NewFinder() returned nil")
	}
	// Check that the returned value is of type *Client.
	if _, ok := interface{}(c).(*Client); !ok {
		t.Errorf("NewFinder() returned %T, expected *Client", c)
	}
}

// cloneRequest returns a shallow copy of the request with a deep copy of headers.
// It is safe to modify the clone without affecting the original request.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of headers
	r2.Header = make(http.Header)
	for k, vv := range r.Header {
		for _, v := range vv {
			r2.Header.Add(k, v)
		}
	}
	// Note: we do not copy the body; assuming it's nil for GET requests.
	return r2
}

// redirectTransport redirects requests to dam.flippenterprise.net to the given test server.
type redirectTransport struct {
	ts          *httptest.Server
	transport   http.RoundTripper
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Host == "dam.flippenterprise.net" || req.URL.Host == "dam.flippenterprise.net" {
		// Clone the request to avoid modifying the original
		r2 := cloneRequest(req)
		// Parse the test server URL to get host and scheme.
		u, err := url.Parse(rt.ts.URL)
		if err != nil {
			return nil, err
		}
		// Replace the host and scheme.
		r2.URL.Host = u.Host
		r2.URL.Scheme = u.Scheme
		// Note: we keep the same Path and Query.
		return rt.transport.RoundTrip(r2)
	}
	// For unexpected hosts, return an error to fail the test.
	return nil, fmt.Errorf("unexpected host: %s, URL.Host: %s", req.Host, req.URL.Host)
}

// newTestClient returns a gentleman client that redirects requests to
// dam.flippenterprise.net to the given test server.
func newTestClient(ts *httptest.Server) *gentleman.Client {
	base := gentleman.New()
	// Create a redirect transport that will redirect dam.flippenterprise.net to ts.
	rt := &redirectTransport{
		ts: ts,
		transport: http.DefaultTransport,
	}
	base.UseRequest(func(ctx *gentlemanv2Context.Context, h gentlemanv2Context.Handler) {
		// Replace the transport of the underlying http.Client with our redirect transport.
		ctx.Client.Transport = rt
		h.Next(ctx)
	})
	return base
}

// TestFindFlyers_BuildsCanonicalFlyers tests that the client converts Flipp flyer DTOs to storage.Flyer.
func TestFindFlyers_BuildsCanonicalFlyers(t *testing.T) {
	// Setup test server with mock Flipp API responses.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/flipp/data" && r.URL.Query().Get("postal_code") == "12345":
			// Mock GetFlyers response.
			resp := GetFlyersResponse{
				Flyers: []Flyer{
					{
						ID:          1,
						Name:        "Test Flyer",
						Merchant:    "Test Merchant",
						ValidFrom:   ptrString("2024-01-01"),
						ValidTo:     ptrString("2024-01-31"),
						FlyerRunID:  0,
						FlyerTypeID: 0,
						Premium:     false,
						Width:       0,
						Height:      0,
						Priority:    0,
						AnalyticsPayload: "",
						MerchantID:    0,
						Path:          "",
						Categories:    []string{},
						Resolutions:   []float64{},
						BudgetID:      nil,
						MerchantLogo:        "",
						PremiumThumbnailURL: "",
						MobileThumbnailURL:  "",
						ThumbnailURL:        "",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/flipp/flyers/1/stores/nearby" && r.URL.Query().Get("postal_code") == "12345":
			// Mock GetNearbyStores response.
			resp := GetStoresNearByResponse{
				{
					ID:         101,
					Address:    "123 Main St",
					City:       "Test City",
					Province:   "TS",
					PostalCode: "12345",
				},
			}
			json.NewEncoder(w).Encode(&resp)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewFinder(WithBaseClient(newTestClient(ts)))

	flyers, err := c.FindFlyers("12345")
	if err != nil {
		t.Fatalf("FindFlyers error: %v", err)
	}

	if len(flyers) != 1 {
		t.Fatalf("expected 1 flyer, got %d", len(flyers))
	}

	f := flyers[0]
	if f.ID != 1 {
		t.Errorf("flyer ID: expected 1, got %d", f.ID)
	}
	if f.Name != "Test Flyer" {
		t.Errorf("flyer name: expected 'Test Flyer', got %q", f.Name)
	}
	if f.Merchant != "Test Merchant" {
		t.Errorf("flyer merchant: expected 'Test Merchant', got %q", f.Merchant)
	}
	// Check dates.
	expectedValidFrom := "2024-01-01"
	if f.ValidFrom != expectedValidFrom {
		t.Errorf("flyer ValidFrom: expected %s, got %s", expectedValidFrom, f.ValidFrom)
	}
	expectedValidTo := "2024-01-31"
	if f.ValidTo != expectedValidTo {
		t.Errorf("flyer ValidTo: expected %s, got %s", expectedValidTo, f.ValidTo)
	}
	// Check stores.
	if len(f.Stores) != 1 {
		t.Errorf("expected 1 store, got %d", len(f.Stores))
	}
	s := f.Stores[0]
	if s.ID != 101 {
		t.Errorf("store ID: expected 101, got %d", s.ID)
	}
	if s.Address != "123 Main St" {
		t.Errorf("store address: expected '123 Main St', got %q", s.Address)
	}
	if s.City != "Test City" {
		t.Errorf("store city: expected 'Test City', got %q", s.City)
	}
	if s.Province != "TS" {
		t.Errorf("store province: expected 'TS', got %q", s.Province)
	}
	if s.PostalCode != "12345" {
		t.Errorf("store postal code: expected '12345', got %q", s.PostalCode)
	}
}

// TestFindFlyers_FallsBackToAvailableDates tests that when valid_from/to are nil, the client uses available_from/to.
func TestFindFlyers_FallsBackToAvailableDates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/flipp/data" && r.URL.Query().Get("postal_code") == "12345":
			resp := GetFlyersResponse{
				Flyers: []Flyer{
					{
						ID:              2,
						Name:            "Fallback Flyer",
						Merchant:        "Test Merchant",
						ValidFrom:       nil, // trigger fallback
						ValidTo:         nil,
						AvailableFrom:   ptrString("2024-02-01"),
						AvailableTo:     ptrString("2024-02-28"),
						FlyerRunID:      0,
						FlyerTypeID:     0,
						Premium:         false,
						Width:           0,
						Height:          0,
						Priority:        0,
						AnalyticsPayload: "",
						MerchantID:      0,
						Path:            "",
						Categories:      []string{},
						Resolutions:     []float64{},
						BudgetID:        nil,
						MerchantLogo:            "",
						PremiumThumbnailURL:     "",
						MobileThumbnailURL:      "",
						ThumbnailURL:            "",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/flipp/flyers/2/stores/nearby" && r.URL.Query().Get("postal_code") == "12345":
			// Mock GetNearbyStores response.
			resp := GetStoresNearByResponse{
				{
					ID:         102,
					Address:    "456 Oak Ave",
					City:       "Another City",
					Province:   "AC",
					PostalCode: "12345",
				},
			}
			json.NewEncoder(w).Encode(&resp)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewFinder(WithBaseClient(newTestClient(ts)))

	flyers, err := c.FindFlyers("12345")
	if err != nil {
		t.Fatalf("FindFlyers error: %v", err)
	}

	if len(flyers) != 1 {
		t.Fatalf("expected 1 flyer, got %d", len(flyers))
	}

	f := flyers[0]
	// Expect ValidFrom/ValidTo to be set from AvailableFrom/AvailableTo.
	expectedFrom := "2024-02-01"
	if f.ValidFrom != expectedFrom {
		t.Errorf("flyer ValidFrom: expected %s (from available_from), got %s", expectedFrom, f.ValidFrom)
	}
	expectedTo := "2024-02-28"
	if f.ValidTo != expectedTo {
		t.Errorf("flyer ValidTo: expected %s (from available_to), got %s", expectedTo, f.ValidTo)
	}
}

// TestFindFlyers_SkipsFlyersWithNoDates tests that a flyer with both valid_from and available_from nil is skipped.
func TestFindFlyers_SkipsFlyersWithNoDates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/flipp/data" && r.URL.Query().Get("postal_code") == "12345":
			resp := GetFlyersResponse{
				Flyers: []Flyer{
					{
						ID:              3,
						Name:            "Skipped Flyer",
						Merchant:        "Test Merchant",
						ValidFrom:       nil,
						ValidTo:         nil,
						AvailableFrom:   nil, // both are nil -> should be skipped
						AvailableTo:     ptrString("2024-03-31"),
						FlyerRunID:      0,
						FlyerTypeID:     0,
						Premium:         false,
						Width:           0,
						Height:          0,
						Priority:        0,
						AnalyticsPayload: "",
						MerchantID:      0,
						Path:            "",
						Categories:      []string{},
						Resolutions:     []float64{},
						BudgetID:        nil,
						MerchantLogo:            "",
						PremiumThumbnailURL:     "",
						MobileThumbnailURL:      "",
						ThumbnailURL:            "",
					},
					{
						ID:              4,
						Name:            "Kept Flyer",
						Merchant:        "Test Merchant",
						ValidFrom:       ptrString("2024-04-01"),
						ValidTo:         ptrString("2024-04-30"),
						FlyerRunID:      0,
						FlyerTypeID:     0,
						Premium:         false,
						Width:           0,
						Height:          0,
						Priority:        0,
						AnalyticsPayload: "",
						MerchantID:      0,
						Path:            "",
						Categories:      []string{},
						Resolutions:     []float64{},
						BudgetID:        nil,
						MerchantLogo:            "",
						PremiumThumbnailURL:     "",
						MobileThumbnailURL:      "",
						ThumbnailURL:            "",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case r.URL.Path == "/api/flipp/flyers/3/stores/nearby" && r.URL.Query().Get("postal_code") == "12345":
			// Mock GetNearbyStores response.
			resp := GetStoresNearByResponse{
				{
					ID:         103,
					Address:    "789 Pine Rd",
					City:       "Third City",
					Province:   "TC",
					PostalCode: "12345",
				},
			}
			json.NewEncoder(w).Encode(&resp)
		case r.URL.Path == "/api/flipp/flyers/4/stores/nearby" && r.URL.Query().Get("postal_code") == "12345":
			// Mock GetNearbyStores response.
			resp := GetStoresNearByResponse{
				{
					ID:         105,
					Address:    "321 Elm St",
					City:       "Fourth City",
					Province:   "FC",
					PostalCode: "12345",
				},
			}
			json.NewEncoder(w).Encode(&resp)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewFinder(WithBaseClient(newTestClient(ts)))

	flyers, err := c.FindFlyers("12345")
	if err != nil {
		t.Fatalf("FindFlyers error: %v", err)
	}

	// Verify that flyers with nil dates (both valid_from and available_from missing)
	// are excluded from the result slice.
	if len(flyers) != 1 {
		t.Fatalf("expected 1 flyer (skipped one with no dates), got %d", len(flyers))
	}
	if flyers[0].ID != 4 {
		t.Errorf("expected kept flyer ID 4, got %d", flyers[0].ID)
	}
}

// TestFindFlyerItems_BuildsCanonicalItems tests that the client converts Flipp flyer item DTOs to storage.FlyerItem.
func TestFindFlyerItems_BuildsCanonicalItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/flipp/flyers/5/flyer_items":
			// Mock GetFlyerItems response.
			resp := GetFlyerItemsResponse{
				{
					ID:             1001,
					FlyerID:        5,
					Name:           "Test Item",
					Brand:          "Test Brand",
					DisplayType:    0,
					Price:          "$4.99",
					CutoutImageURL: "http://example.com/image.jpg",
					VideoURL:       ptrString("http://example.com/video.mp4"),
					ValidFrom:      "",
					ValidTo:        "",
					AvailableTo:    "",
					Left:           0,
					Right:          0,
					Top:            0,
					Bottom:         0,
					PageDestination: nil,
				},
				{
					ID:             1002,
					FlyerID:        5,
					Name:           "Another Item",
					Brand:          "",
					DisplayType:    1,
					Price:          "",
					CutoutImageURL: "",
					VideoURL:       nil,
					ValidFrom:      "",
					ValidTo:        "",
					AvailableTo:    "",
					Left:           0,
					Right:          0,
					Top:            0,
					Bottom:         0,
					PageDestination: nil,
				},
			}
			json.NewEncoder(w).Encode(&resp)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewFinder(WithBaseClient(newTestClient(ts)))

	items, err := c.FindFlyerItems(5)
	if err != nil {
		t.Fatalf("FindFlyerItems error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First item with video URL.
	it1 := items[0]
	if it1.ID != 1001 {
		t.Errorf("first item ID: expected 1001, got %d", it1.ID)
	}
	if it1.FlyerID != 5 {
		t.Errorf("first item FlyerID: expected 5, got %d", it1.FlyerID)
	}
	if it1.Name != "Test Item" {
		t.Errorf("first item name: expected 'Test Item', got %q", it1.Name)
	}
	if it1.Brand != "Test Brand" {
		t.Errorf("first item brand: expected 'Test Brand', got %q", it1.Brand)
	}
	if it1.Price != "$4.99" {
		t.Errorf("first item price: expected '$4.99', got %q", it1.Price)
	}
	if it1.CutoutImageURL != "http://example.com/image.jpg" {
		t.Errorf("first item CutoutImageURL: expected 'http://example.com/image.jpg', got %q", it1.CutoutImageURL)
	}
	if it1.VideoURL == nil {
		t.Error("first item VideoURL: expected non-nil pointer")
	} else if *it1.VideoURL != "http://example.com/video.mp4" {
		t.Errorf("first item VideoURL: expected 'http://example.com/video.mp4', got %q", *it1.VideoURL)
	}

	// Second item with nil video URL.
	it2 := items[1]
	if it2.ID != 1002 {
		t.Errorf("second item ID: expected 1002, got %d", it2.ID)
	}
	if it2.VideoURL != nil {
		t.Error("second item VideoURL: expected nil, got non-nil")
	}
}

// TestFindFlyerItems_NilVideoURL tests that when the API sends null for video_url, the client sets VideoURL to nil.
func TestFindFlyerItems_NilVideoURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/flipp/flyers/6/flyer_items":
			resp := GetFlyerItemsResponse{
				{
					ID:             2001,
					FlyerID:        6,
					Name:           "Item with null video",
					Brand:          "Test Brand",
					DisplayType:    0,
					Price:          "$5.99",
					CutoutImageURL: "http://example.com/image2.jpg",
					VideoURL:       nil, // explicit nil in struct -> JSON null
					ValidFrom:      "",
					ValidTo:        "",
					AvailableTo:    "",
					Left:           0,
					Right:          0,
					Top:            0,
					Bottom:         0,
					PageDestination: nil,
				},
			}
			json.NewEncoder(w).Encode(&resp)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c := NewFinder(WithBaseClient(newTestClient(ts)))

	items, err := c.FindFlyerItems(6)
	if err != nil {
		t.Fatalf("FindFlyerItems error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0].VideoURL != nil {
		t.Error("expected VideoURL to be nil when API sends nil")
	}
}