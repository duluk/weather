package openmeteo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/duluk/weather/pkg/weather"
)

/* --> Response to GetCurrentWeather:
{
  "latitude": 32.33599,
  "longitude": -90.32324,
  "generationtime_ms": 0.06186962127685547,
  "utc_offset_seconds": 0,
  "timezone": "GMT",
  "timezone_abbreviation": "GMT",
  "elevation": 112.0,
  "current_units": {
    "time": "iso8601",
    "interval": "seconds",
    "temperature_2m": "°F",
    "relativehumidity_2m": "%",
    "weathercode": "wmo code",
    "windspeed_10m": "km/h"
  },
  "current": {
    "time": "2025-02-15T03:30",
    "interval": 900,
    "temperature_2m": 52.8,
    "relativehumidity_2m": 62,
    "weathercode": 3,
    "windspeed_10m": 12.5
  }
}
*/

type WeatherResponse struct {
	CurrentWeather struct {
		Temperature      float64 `json:"temperature_2m"`
		WindSpeed        float64 `json:"windspeed_10m"`
		WeatherCode      int     `json:"weathercode"`
		RelativeHumidity int     `json:"relativehumidity_2m"`
	} `json:"current"`
	Daily struct {
		Time             []string  `json:"time"`
		TempMax          []float64 `json:"temperature_2m_max"`
		TempMin          []float64 `json:"temperature_2m_min"`
		WindSpeed        []float64 `json:"windspeed_10m_max"`
		WeatherCode      []int     `json:"weathercode"`
		RelativeHumidity []int     `json:"relative_humidity_2m_max"`
	} `json:"daily"`
}

type Provider struct {
	debugMode bool
}

/* Example Geocoding structure response:
{
  "id": 4852022,
  "name": "Clinton",
  "latitude": 41.84447,
  "longitude": -90.18874,
  "elevation": 179,
  "feature_code": "PPLA2",
  "country_code": "US",
  "admin1_id": 4862182,
  "admin2_id": 4852032,
  "admin3_id": 4852053,
  "timezone": "America/Chicago",
  "population": 26064,
  "postcodes": [
    "52732",
    "52733",
    "52736",
    "52734"
  ],
  "country_id": 6252001,
  "country": "United States",
  "admin1": "Iowa",
  "admin2": "Clinton",
  "admin3": "City of Clinton"
}
*/

type GeocodingResult struct {
	Name      string  `json:"name"`
	State     string  `json:"admin1"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type GeocodingResponse struct {
	Results []GeocodingResult `json:"results"`
}

func (p *Provider) getCoordinates(location string) (*GeocodingResult, error) {
	var count int
	var state string
	if regexp.MustCompile(`^[0-9]{5}$`).MatchString(location) {
		location = fmt.Sprintf("%s", location)
		count = 1
	} else if regexp.MustCompile(`^[a-zA-Z ]+, ?[A-Z]{2}$`).MatchString(location) {
		parts := strings.Split(location, ",")
		if len(parts) == 2 {
			city := strings.TrimSpace(parts[0])
			state = strings.TrimSpace(parts[1])
			location = fmt.Sprintf("%s", city)
		}
		count = 10
	} else {
		location = fmt.Sprintf("%s", location)
	}

	url := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=%d&language=en&format=json",
		url.QueryEscape(location), count)

	var data GeocodingResponse
	if err := p.fetchData(url, &data); err != nil {
		return nil, err
	}

	if len(data.Results) == 0 {
		return nil, fmt.Errorf("location not found: %s", location)
	}

	// Open-Meteo API doesn't allow the state in the query but returns it in
	// the response, so we have to match it ourselves. That is, it wil return
	// all cities that match the name, so we have to filter by state.
	if state != "" {
		for _, result := range data.Results {
			if matchedState(result.State, state) {
				return &result, nil
			}
		}
		return nil, fmt.Errorf("location not found: %s", location)
	}

	return &data.Results[0], nil
}

func New(debugMode bool) *Provider {
	return &Provider{debugMode: debugMode}
}

func (p *Provider) GetCurrentWeather(location string) (*weather.CurrentWeather, error) {
	coords, err := p.getCoordinates(location)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,relativehumidity_2m,weathercode,windspeed_10m&temperature_unit=fahrenheit&timezone=auto&forecast_days=1&daily=temperature_2m_max,temperature_2m_min",
		coords.Latitude, coords.Longitude)
	if p.debugMode {
		fmt.Printf("Debug GetCurrentWeather URL: %s\n", url)
	}

	var data WeatherResponse
	if err := p.fetchData(url, &data); err != nil {
		return nil, err
	}
	if p.debugMode {
		fmt.Printf("Debug GetCurrentWeather data: %+v\n", data)
	}

	var highTemp, lowTemp float64
	if len(data.Daily.TempMax) > 0 && len(data.Daily.TempMin) > 0 {
		highTemp = data.Daily.TempMax[0]
		lowTemp = data.Daily.TempMin[0]
	}

	return &weather.CurrentWeather{
		Location:    coords.Name,
		Conditions:  p.getWeatherDescription(data.CurrentWeather.WeatherCode),
		Temperature: data.CurrentWeather.Temperature,
		FeelsLike:   data.CurrentWeather.Temperature,
		Humidity:    data.CurrentWeather.RelativeHumidity,
		WindSpeed:   data.CurrentWeather.WindSpeed,
		TempMax:     highTemp,
		TempMin:     lowTemp,
	}, nil
}

func (p *Provider) GetForecast(location string) (*weather.Forecast, error) {
	coords, err := p.getCoordinates(location)
	if err != nil {
		return nil, err
	}

	// Request 6 days to get enough data (today + 5 future days)
	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&daily=weathercode,temperature_2m_max,temperature_2m_min,windspeed_10m_max,relative_humidity_2m_max&current=temperature_2m,relativehumidity_2m,weathercode,windspeed_10m&temperature_unit=fahrenheit&timezone=auto&forecast_days=6",
		coords.Latitude, coords.Longitude)

	if p.debugMode {
		fmt.Printf("Debug GetForecast URL: %s\n", url)
	}

	var data WeatherResponse
	if err := p.fetchData(url, &data); err != nil {
		return nil, err
	}

	if len(data.Daily.Time) < 2 {
		return nil, fmt.Errorf("insufficient forecast data available")
	}

	dailyItems := make([]weather.DailyForecast, 5)
	for i := 0; i < 5; i++ {
		sourceIdx := i + 1 // Skip the first, current, day
		date, _ := time.Parse("2006-01-02", data.Daily.Time[sourceIdx])
		dailyItems[i] = weather.DailyForecast{
			Date:       date,
			Conditions: p.getWeatherDescription(data.Daily.WeatherCode[sourceIdx]),
			High:       data.Daily.TempMax[sourceIdx],
			Low:        data.Daily.TempMin[sourceIdx],
			WindSpeed:  data.Daily.WindSpeed[sourceIdx],
			Humidity:   data.Daily.RelativeHumidity[sourceIdx],
		}
	}

	var highTemp, lowTemp float64
	if len(data.Daily.TempMax) > 0 && len(data.Daily.TempMin) > 0 {
		highTemp = data.Daily.TempMax[0]
		lowTemp = data.Daily.TempMin[0]
	}

	current := &weather.CurrentWeather{
		Location:    coords.Name,
		Conditions:  p.getWeatherDescription(data.CurrentWeather.WeatherCode),
		Temperature: data.CurrentWeather.Temperature,
		FeelsLike:   data.CurrentWeather.Temperature,
		Humidity:    data.CurrentWeather.RelativeHumidity,
		WindSpeed:   data.CurrentWeather.WindSpeed,
		TempMax:     highTemp,
		TempMin:     lowTemp,
	}

	return &weather.Forecast{
		Location:   coords.Name,
		Current:    current,
		DailyItems: dailyItems,
	}, nil
}

func (p *Provider) fetchData(url string, target interface{}) error {
	if p.debugMode {
		fmt.Printf("Debug fetchData URL: %s\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}
	if p.debugMode {
		fmt.Printf("Debug fetchData response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(body))
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	return nil
}

func (p *Provider) getWeatherDescription(code int) string {
	// WMO Weather interpretation codes (https://open-meteo.com/en/docs)
	codes := map[int]string{
		0:  "clear sky",
		1:  "mainly clear",
		2:  "partly cloudy",
		3:  "overcast",
		45: "foggy",
		48: "depositing rime fog",
		51: "light drizzle",
		53: "moderate drizzle",
		55: "dense drizzle",
		61: "slight rain",
		63: "moderate rain",
		65: "heavy rain",
		71: "slight snow",
		73: "moderate snow",
		75: "heavy snow",
		77: "snow grains",
		80: "slight rain showers",
		81: "moderate rain showers",
		82: "violent rain showers",
		85: "slight snow showers",
		86: "heavy snow showers",
		95: "thunderstorm",
		96: "thunderstorm with slight hail",
		99: "thunderstorm with heavy hail",
	}

	if desc, ok := codes[code]; ok {
		return desc
	}
	return "unknown"
}

func matchedState(fullName, abbrev string) bool {
	stateMap := map[string]string{
		"Alabama":        "AL",
		"Alaska":         "AK",
		"Arizona":        "AZ",
		"Arkansas":       "AR",
		"California":     "CA",
		"Colorado":       "CO",
		"Connecticut":    "CT",
		"Delaware":       "DE",
		"Florida":        "FL",
		"Georgia":        "GA",
		"Hawaii":         "HI",
		"Idaho":          "ID",
		"Illinois":       "IL",
		"Indiana":        "IN",
		"Iowa":           "IA",
		"Kansas":         "KS",
		"Kentucky":       "KY",
		"Louisiana":      "LA",
		"Maine":          "ME",
		"Maryland":       "MD",
		"Massachusetts":  "MA",
		"Michigan":       "MI",
		"Minnesota":      "MN",
		"Mississippi":    "MS",
		"Missouri":       "MO",
		"Montana":        "MT",
		"Nebraska":       "NE",
		"Nevada":         "NV",
		"New Hampshire":  "NH",
		"New Jersey":     "NJ",
		"New Mexico":     "NM",
		"New York":       "NY",
		"North Carolina": "NC",
		"North Dakota":   "ND",
		"Ohio":           "OH",
		"Oklahoma":       "OK",
		"Oregon":         "OR",
		"Pennsylvania":   "PA",
		"Rhode Island":   "RI",
		"South Carolina": "SC",
		"South Dakota":   "SD",
		"Tennessee":      "TN",
		"Texas":          "TX",
		"Utah":           "UT",
		"Vermont":        "VT",
		"Virginia":       "VA",
		"Washington":     "WA",
		"West Virginia":  "WV",
		"Wisconsin":      "WI",
		"Wyoming":        "WY",
	}

	if abbr, ok := stateMap[fullName]; ok {
		return abbr == abbrev
	}

	return false
}
