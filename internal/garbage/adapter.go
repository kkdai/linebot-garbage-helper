package garbage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"linebot-garbage-helper/internal/geo"
)

type GarbageAdapter struct {
	httpClient *http.Client
}

type GarbageData struct {
	Routes []Route `json:"routes"`
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
}

func NewGarbageAdapter() *GarbageAdapter {
	return &GarbageAdapter{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (ga *GarbageAdapter) FetchGarbageData(ctx context.Context) (*GarbageData, error) {
	url := "https://raw.githubusercontent.com/Yukaii/garbage/main/garbage.json"
	
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
	now := time.Now()
	
	for _, route := range data.Routes {
		for _, stop := range route.Stops {
			distance := geo.CalculateDistance(userLat, userLng, stop.Lat, stop.Lng)
			
			eta, err := parseTimeToToday(stop.Time)
			if err != nil {
				continue
			}
			
			if eta.Before(now) {
				eta = eta.Add(24 * time.Hour)
			}
			
			nearestStops = append(nearestStops, &NearestStop{
				Stop:     stop,
				Route:    route,
				Distance: distance,
				ETA:      eta,
			})
		}
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
	
	for _, route := range data.Routes {
		for _, stop := range route.Stops {
			distance := geo.CalculateDistance(userLat, userLng, stop.Lat, stop.Lng)
			
			if maxDistance > 0 && distance > maxDistance {
				continue
			}
			
			eta, err := parseTimeToToday(stop.Time)
			if err != nil {
				continue
			}
			
			if eta.Before(time.Now()) {
				eta = eta.Add(24 * time.Hour)
			}
			
			if !isTimeInWindow(eta, timeWindow) {
				continue
			}
			
			validStops = append(validStops, &NearestStop{
				Stop:     stop,
				Route:    route,
				Distance: distance,
				ETA:      eta,
			})
		}
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
	now := time.Now()
	layout := "15:04"
	
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), nil
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

func (ga *GarbageAdapter) GetRouteByID(data *GarbageData, routeID string) *Route {
	for _, route := range data.Routes {
		if route.ID == routeID {
			return &route
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