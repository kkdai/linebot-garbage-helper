package geo

import (
	"context"
	"fmt"
	"math"

	"googlemaps.github.io/maps"
)

type GeocodeClient struct {
	client *maps.Client
}

type Location struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Address string  `json:"address"`
}

func NewGeocodeClient(apiKey string) (*GeocodeClient, error) {
	client, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &GeocodeClient{client: client}, nil
}

func (gc *GeocodeClient) GeocodeAddress(ctx context.Context, address string) (*Location, error) {
	req := &maps.GeocodingRequest{
		Address: address,
	}
	
	resp, err := gc.client.Geocode(ctx, req)
	if err != nil {
		return nil, err
	}
	
	if len(resp) == 0 {
		return nil, fmt.Errorf("no results found for address: %s", address)
	}
	
	result := resp[0]
	return &Location{
		Lat:     result.Geometry.Location.Lat,
		Lng:     result.Geometry.Location.Lng,
		Address: result.FormattedAddress,
	}, nil
}

func (gc *GeocodeClient) ReverseGeocode(ctx context.Context, lat, lng float64) (*Location, error) {
	req := &maps.GeocodingRequest{
		LatLng: &maps.LatLng{
			Lat: lat,
			Lng: lng,
		},
	}
	
	resp, err := gc.client.Geocode(ctx, req)
	if err != nil {
		return nil, err
	}
	
	if len(resp) == 0 {
		return nil, fmt.Errorf("no results found for coordinates: %f, %f", lat, lng)
	}
	
	result := resp[0]
	return &Location{
		Lat:     lat,
		Lng:     lng,
		Address: result.FormattedAddress,
	}, nil
}

func (gc *GeocodeClient) GetDirectionsURL(lat, lng float64) string {
	return fmt.Sprintf("https://maps.google.com/?q=%f,%f", lat, lng)
}

func CalculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371000

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func FormatDistance(meters float64) string {
	if meters < 1000 {
		return fmt.Sprintf("約%.0f公尺", meters)
	}
	return fmt.Sprintf("約%.1f公里", meters/1000)
}