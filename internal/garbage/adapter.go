package garbage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"linebot-garbage-helper/internal/geo"
	"linebot-garbage-helper/internal/utils"
)

type GarbageAdapter struct {
	httpClient *http.Client
}

type GarbageData struct {
	Result GarbageResult `json:"result"`
}

type GarbageResult struct {
	Count   int               `json:"count"`
	Limit   int               `json:"limit"`
	Offset  int               `json:"offset"`
	Sort    string            `json:"sort"`
	Results []CollectionPoint `json:"results"`
}

type CollectionPoint struct {
	ID          int         `json:"_id"`
	ImportDate  ImportDate  `json:"_importdate"`
	District    string      `json:"行政區"`
	Neighborhood string     `json:"里別"`
	Squad       string      `json:"分隊"`
	StationCode string      `json:"局編"`
	VehicleNumber string    `json:"車號"`
	Route       string      `json:"路線"`
	VehicleTrip string      `json:"車次"`
	ArrivalTime string      `json:"抵達時間"`
	DepartureTime string    `json:"離開時間"`
	Location    string      `json:"地點"`
	Longitude   string      `json:"經度"`
	Latitude    string      `json:"緯度"`
}

type ImportDate struct {
	Date         string `json:"date"`
	TimezoneType int    `json:"timezone_type"`
	Timezone     string `json:"timezone"`
}

type Route struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Stops []Stop `json:"stops"`
}

type Stop struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Time string  `json:"time"`
}

type NearestStop struct {
	Stop     Stop
	Route    Route
	Distance float64
	ETA      time.Time
	CollectionPoint *CollectionPoint
}

func NewGarbageAdapter() *GarbageAdapter {
	return &GarbageAdapter{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (ga *GarbageAdapter) FetchGarbageData(ctx context.Context) (*GarbageData, error) {
	url := "https://raw.githubusercontent.com/Yukaii/garbage/data/trash-collection-points.json"
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := ga.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	var data GarbageData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	
	return &data, nil
}

func (ga *GarbageAdapter) FindNearestStops(userLat, userLng float64, data *GarbageData, limit int) ([]*NearestStop, error) {
	var nearestStops []*NearestStop
	now := utils.NowInTaiwan()
	
	for _, point := range data.Result.Results {
		lat, lng, err := ga.parseCoordinates(point.Latitude, point.Longitude)
		if err != nil {
			continue
		}
		
		distance := geo.CalculateDistance(userLat, userLng, lat, lng)
		
		eta, err := parseTimeToToday(point.ArrivalTime)
		if err != nil {
			continue
		}
		
		// 使用台灣時區比較時間
		if eta.Before(now) {
			eta = eta.Add(24 * time.Hour)
		}
		
		stop := Stop{
			Name: point.Location,
			Lat:  lat,
			Lng:  lng,
			Time: point.ArrivalTime,
		}
		
		route := Route{
			ID:   point.VehicleNumber,
			Name: point.Route,
		}
		
		nearestStops = append(nearestStops, &NearestStop{
			Stop:            stop,
			Route:           route,
			Distance:        distance,
			ETA:             eta,
			CollectionPoint: &point,
		})
	}
	
	sort.Slice(nearestStops, func(i, j int) bool {
		return nearestStops[i].Distance < nearestStops[j].Distance
	})
	
	if limit > 0 && len(nearestStops) > limit {
		nearestStops = nearestStops[:limit]
	}
	
	return nearestStops, nil
}

func (ga *GarbageAdapter) FindStopsInTimeWindow(userLat, userLng float64, data *GarbageData, timeWindow TimeWindow, maxDistance float64) ([]*NearestStop, error) {
	var validStops []*NearestStop
	now := utils.NowInTaiwan()
	
	for _, point := range data.Result.Results {
		lat, lng, err := ga.parseCoordinates(point.Latitude, point.Longitude)
		if err != nil {
			continue
		}
		
		distance := geo.CalculateDistance(userLat, userLng, lat, lng)
		
		if maxDistance > 0 && distance > maxDistance {
			continue
		}
		
		eta, err := parseTimeToToday(point.ArrivalTime)
		if err != nil {
			continue
		}
		
		if eta.Before(now) {
			eta = eta.Add(24 * time.Hour)
		}
		
		if !isTimeInWindow(eta, timeWindow) {
			continue
		}
		
		stop := Stop{
			Name: point.Location,
			Lat:  lat,
			Lng:  lng,
			Time: point.ArrivalTime,
		}
		
		route := Route{
			ID:   point.VehicleNumber,
			Name: point.Route,
		}
		
		validStops = append(validStops, &NearestStop{
			Stop:            stop,
			Route:           route,
			Distance:        distance,
			ETA:             eta,
			CollectionPoint: &point,
		})
	}
	
	sort.Slice(validStops, func(i, j int) bool {
		return validStops[i].ETA.Before(validStops[j].ETA)
	})
	
	return validStops, nil
}

type TimeWindow struct {
	From time.Time
	To   time.Time
}

func parseTimeToToday(timeStr string) (time.Time, error) {
	taipeiTZ := utils.GetTaiwanTimezone()
	now := utils.NowInTaiwan()
	
	if len(timeStr) == 4 {
		layout := "1504"
		t, err := time.Parse(layout, timeStr)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, taipeiTZ), nil
	}
	
	layout := "15:04"
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, taipeiTZ), nil
}

func isTimeInWindow(t time.Time, window TimeWindow) bool {
	if window.From.IsZero() && window.To.IsZero() {
		return true
	}
	
	if !window.From.IsZero() && t.Before(window.From) {
		return false
	}
	
	if !window.To.IsZero() && t.After(window.To) {
		return false
	}
	
	return true
}

func (ga *GarbageAdapter) parseCoordinates(lat, lng string) (float64, float64, error) {
	latFloat, err := strconv.ParseFloat(strings.TrimSpace(lat), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %s", lat)
	}
	
	lngFloat, err := strconv.ParseFloat(strings.TrimSpace(lng), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %s", lng)
	}
	
	return latFloat, lngFloat, nil
}

func (ga *GarbageAdapter) GetRouteByID(data *GarbageData, routeID string) *Route {
	for _, point := range data.Result.Results {
		if point.VehicleNumber == routeID {
			lat, lng, err := ga.parseCoordinates(point.Latitude, point.Longitude)
			if err != nil {
				continue
			}
			
			stop := Stop{
				Name: point.Location,
				Lat:  lat,
				Lng:  lng,
				Time: point.ArrivalTime,
			}
			
			return &Route{
				ID:    point.VehicleNumber,
				Name:  point.Route,
				Stops: []Stop{stop},
			}
		}
	}
	return nil
}

func (ga *GarbageAdapter) GetStopFromRoute(route *Route, stopName string) *Stop {
	for _, stop := range route.Stops {
		if stop.Name == stopName {
			return &stop
		}
	}
	return nil
}

func (ga *GarbageAdapter) GetCollectionPointByVehicleAndLocation(data *GarbageData, vehicleNumber, location string) *CollectionPoint {
	for _, point := range data.Result.Results {
		if point.VehicleNumber == vehicleNumber && point.Location == location {
			return &point
		}
	}
	return nil
}