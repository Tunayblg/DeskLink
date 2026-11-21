// Package weather reads the weather of one city from open-meteo.com (free, no
// key). Only the city name the user typed on the phone leaves the PC.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Report is the weather now in one place.
type Report struct {
	City  string  `json:"city"`
	Temp  float64 `json:"temp"`  // °C
	Feels float64 `json:"feels"` // °C
	Min   float64 `json:"min"`   // today
	Max   float64 `json:"max"`   // today
	Wind  float64 `json:"wind"`  // km/h
	Code  int     `json:"code"`  // WMO weather code
	Day   bool    `json:"day"`
}

const (
	maxCity   = 60
	cacheTime = 10 * time.Minute
)

// Client asks open-meteo and keeps the last answer of each city for a while,
// so several phones (or a phone asking again) cost one request.
type Client struct {
	http *http.Client
	geo  string
	api  string

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	at time.Time
	r  Report
}

// New returns a client with a short timeout.
func New() *Client {
	return &Client{
		http:  &http.Client{Timeout: 8 * time.Second},
		geo:   "https://geocoding-api.open-meteo.com/v1/search",
		api:   "https://api.open-meteo.com/v1/forecast",
		cache: map[string]cached{},
	}
}

// Get returns the weather for a city name.
func (c *Client) Get(ctx context.Context, city string) (Report, error) {
	city = strings.TrimSpace(city)
	if city == "" || utf8.RuneCountInString(city) > maxCity {
		return Report{}, errors.New("bad city")
	}
	key := strings.ToLower(city)
	c.mu.Lock()
	if e, ok := c.cache[key]; ok && time.Since(e.at) < cacheTime {
		c.mu.Unlock()
		return e.r, nil
	}
	c.mu.Unlock()

	var geo struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	q := url.Values{"name": {city}, "count": {"1"}, "format": {"json"}}
	if err := c.getJSON(ctx, c.geo+"?"+q.Encode(), &geo); err != nil {
		return Report{}, err
	}
	if len(geo.Results) == 0 {
		return Report{}, errors.New("not found")
	}
	place := geo.Results[0]

	var fc struct {
		Current struct {
			Temp  float64 `json:"temperature_2m"`
			Feels float64 `json:"apparent_temperature"`
			Code  int     `json:"weather_code"`
			Day   int     `json:"is_day"`
			Wind  float64 `json:"wind_speed_10m"`
		} `json:"current"`
		Daily struct {
			Max []float64 `json:"temperature_2m_max"`
			Min []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	q = url.Values{
		"latitude":      {fmt.Sprintf("%.4f", place.Latitude)},
		"longitude":     {fmt.Sprintf("%.4f", place.Longitude)},
		"current":       {"temperature_2m,apparent_temperature,weather_code,is_day,wind_speed_10m"},
		"daily":         {"temperature_2m_max,temperature_2m_min"},
		"timezone":      {"auto"},
		"forecast_days": {"1"},
	}
	if err := c.getJSON(ctx, c.api+"?"+q.Encode(), &fc); err != nil {
		return Report{}, err
	}
	r := Report{City: place.Name, Temp: fc.Current.Temp, Feels: fc.Current.Feels, Code: fc.Current.Code,
		Day: fc.Current.Day == 1, Wind: fc.Current.Wind}
	if len(fc.Daily.Max) > 0 && len(fc.Daily.Min) > 0 {
		r.Max, r.Min = fc.Daily.Max[0], fc.Daily.Min[0]
	} else {
		r.Max, r.Min = r.Temp, r.Temp
	}

	c.mu.Lock()
	c.cache[key] = cached{at: time.Now(), r: r}
	c.mu.Unlock()
	return r, nil
}

func (c *Client) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "DeskLink")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("weather: %s", resp.Status)
	}
	return json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(out)
}
