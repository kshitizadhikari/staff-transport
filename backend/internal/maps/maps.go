// Package maps wraps external mapping providers behind interfaces so domain
// modules never depend on a specific provider.
package maps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound indicates the provider returned no match for an address.
var ErrNotFound = errors.New("no geocoding result")

// ErrNotConfigured indicates no provider token is configured.
var ErrNotConfigured = errors.New("geocoding is not configured")

// Location is a geocoded address.
type Location struct {
	Address   string
	Latitude  float64
	Longitude float64
}

// Geocoder resolves a human-readable address to coordinates.
type Geocoder interface {
	Geocode(ctx context.Context, address string) (*Location, error)
}

const mapboxEndpoint = "https://api.mapbox.com/geocoding/v5/mapbox.places"

// MapboxGeocoder geocodes addresses with the Mapbox Geocoding API.
type MapboxGeocoder struct {
	client   *http.Client
	token    string
	endpoint string
	country  string
}

// NewMapboxGeocoder returns a Mapbox-backed geocoder. country is an optional
// ISO 3166-1 alpha-2 code used to bias results.
func NewMapboxGeocoder(token, country string) *MapboxGeocoder {
	return &MapboxGeocoder{
		client:   &http.Client{Timeout: 5 * time.Second},
		token:    token,
		endpoint: mapboxEndpoint,
		country:  country,
	}
}

type mapboxResponse struct {
	Features []struct {
		PlaceName string    `json:"place_name"`
		Center    []float64 `json:"center"`
	} `json:"features"`
}

func (g *MapboxGeocoder) Geocode(ctx context.Context, address string) (*Location, error) {
	if g == nil || g.token == "" {
		return nil, ErrNotConfigured
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, ErrNotFound
	}

	requestURL, err := url.Parse(g.endpoint + "/" + url.PathEscape(address) + ".json")
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("access_token", g.token)
	query.Set("limit", strconv.Itoa(1))
	if g.country != "" {
		query.Set("country", g.country)
	}
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := g.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("mapbox geocoding unauthorized (status %d)", response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mapbox geocoding failed with status %d", response.StatusCode)
	}

	var payload mapboxResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Features) == 0 || len(payload.Features[0].Center) < 2 {
		return nil, ErrNotFound
	}

	feature := payload.Features[0]
	return &Location{
		Address:   feature.PlaceName,
		Longitude: feature.Center[0],
		Latitude:  feature.Center[1],
	}, nil
}
