package openweather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/duluk/weather/pkg/weather"
)

/*
	OpenWeather API Response Codes
	Success codes
	200  // Success for current weather data
	201  // Success for forecast data

	Error codes
	400  // Bad request (e.g., invalid parameters)
	401  // Unauthorized (invalid API key)
	404  // City not found
	429  // Too many requests (exceeded rate limit)
	500  // Internal server error
*/

type WeatherData struct {
	DateTime    int64 `json:"dt"`
	TimeZone    int   `json:"timezone"`
	Coordinates struct {
		Latitude  float64 `json:"lat"`
		Longitude float64 `json:"lon"`
	} `json:"coord"`
	Base    string `json:"base"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp        float64 `json:"temp"`
		TempMin     float64 `json:"temp_min"`
		TempMax     float64 `json:"temp_max"`
		FeelsLike   float64 `json:"feels_like"`
		Humidity    int     `json:"humidity"`
		Pressure    int     `json:"pressure"`
		SeaLevel    int     `json:"sea_level"`
		GroundLevel int     `json:"grnd_level"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"`
		Gust  float64 `json:"gust"`
		Deg   int     `json:"deg"`
	} `json:"wind"`
	Clouds struct {
		Percentage int `json:"all"`
	} `json:"clouds"`
	Sys struct {
		Sunrise int64 `json:"sunrise"`
		Sunset  int64 `json:"sunset"`
	} `json:"sys"`
	Visibility int    `json:"visibility"`
	Name       string `json:"name"`
	RespCode   int    `json:"cod"`
}

type ForecastData struct {
	Count int `json:"cnt"`
	List  []struct {
		DateTime int64 `json:"dt"`
		Main     struct {
			Temp        float64 `json:"temp"`
			TempMin     float64 `json:"temp_min"`
			TempMax     float64 `json:"temp_max"`
			FeelsLike   float64 `json:"feels_like"`
			Humidity    int     `json:"humidity"`
			Pressure    int     `json:"pressure"`
			SeaLevel    int     `json:"sea_level"`
			GroundLevel int     `json:"grnd_level"`
			TempKf      float64 `json:"temp_kf"`
		} `json:"main"`
		Weather []struct {
			Description string `json:"description"`
		} `json:"weather"`
		Clouds struct {
			Percentage int `json:"all"`
		} `json:"clouds"`
		Wind struct {
			Speed float64 `json:"speed"`
			Gust  float64 `json:"gust"`
			Deg   int     `json:"deg"`
		} `json:"wind"`
		DateText   string `json:"dt_txt"`
		Visibility int    `json:"visibility"`
	} `json:"list"`
	City struct {
		Name        string `json:"name"`
		Coordinates struct {
			Latitude  float64 `json:"lat"`
			Longitude float64 `json:"lon"`
		} `json:"coord"`
		Country    string `json:"country"`
		Population int    `json:"population"`
		TimeZone   int    `json:"timezone"`
		Sunrise    int64  `json:"sunrise"`
		Sunset     int64  `json:"sunset"`
	} `json:"city"`
	RespCode string `json:"cod"`
}

type Provider struct {
	apiKey      string
	useTestData bool
	debugMode   bool
}

func New(apiKey string, useTestData, debugMode bool) *Provider {
	return &Provider{
		apiKey:      apiKey,
		useTestData: useTestData,
		debugMode:   debugMode,
	}
}

func (p *Provider) GetCurrentWeather(location string) (*weather.CurrentWeather, error) {
	var data WeatherData
	if err := p.fetchData(location, false, &data); err != nil {
		return nil, err
	}

	if len(data.Weather) == 0 {
		return nil, fmt.Errorf("no weather data available")
	}

	return &weather.CurrentWeather{
		Location:    data.Name,
		Conditions:  data.Weather[0].Description,
		Temperature: data.Main.Temp,
		FeelsLike:   data.Main.FeelsLike,
		TempMax:     data.Main.TempMax,
		TempMin:     data.Main.TempMin,
		Humidity:    data.Main.Humidity,
		WindSpeed:   data.Wind.Speed,
	}, nil
}

func (p *Provider) GetForecast(location string) (*weather.Forecast, error) {
	var data ForecastData
	if err := p.fetchData(location, true, &data); err != nil {
		return nil, err
	}

	if len(data.List) == 0 {
		return nil, fmt.Errorf("no forecast data available")
	}

	forecast := &weather.Forecast{
		Location:   data.City.Name,
		Current:    p.getCurrentFromForecast(&data),
		DailyItems: p.processForecastData(&data),
	}

	return forecast, nil
}

func (p *Provider) getCurrentFromForecast(data *ForecastData) *weather.CurrentWeather {
	if len(data.List) == 0 || len(data.List[0].Weather) == 0 {
		return nil
	}

	current := data.List[0]
	return &weather.CurrentWeather{
		Location:    data.City.Name,
		Conditions:  current.Weather[0].Description,
		Temperature: current.Main.Temp,
		FeelsLike:   current.Main.FeelsLike,
		TempMax:     current.Main.TempMax,
		TempMin:     current.Main.TempMin,
		Humidity:    current.Main.Humidity,
		WindSpeed:   current.Wind.Speed,
	}
}

func (p *Provider) processForecastData(data *ForecastData) []weather.DailyForecast {
	type dailyData struct {
		high        float64
		low         float64
		description string
		windSpeed   float64
		humidity    int
	}

	dailyForecasts := make(map[string]*dailyData)

	for _, item := range data.List {
		date := strings.Split(item.DateText, " ")[0]
		time := strings.Split(item.DateText, " ")[1]

		// Only process readings between 06:00 and 00:00
		hour := strings.Split(time, ":")[0]
		if hour < "06" {
			continue
		}

		if _, exists := dailyForecasts[date]; !exists {
			dailyForecasts[date] = &dailyData{
				high:        -1000,
				low:         1000,
				description: item.Weather[0].Description,
				windSpeed:   0,
				humidity:    item.Main.Humidity,
			}
		}

		day := dailyForecasts[date]
		if item.Main.TempMax > day.high {
			day.high = item.Main.TempMax
		}
		if item.Main.TempMin < day.low {
			day.low = item.Main.TempMin
		}
		if item.Wind.Speed > day.windSpeed {
			day.windSpeed = item.Wind.Speed
		}

		if strings.Contains(item.DateText, "12:00:00") {
			day.description = item.Weather[0].Description
			day.humidity = item.Main.Humidity
		}
	}

	// Convert to sorted slice
	var dates []string
	for date := range dailyForecasts {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	result := make([]weather.DailyForecast, 0, len(dates))
	for _, date := range dates {
		day := dailyForecasts[date]
		parsedDate, _ := time.Parse("2006-01-02", date)
		result = append(result, weather.DailyForecast{
			Date:       parsedDate,
			Conditions: day.description,
			High:       day.high,
			Low:        day.low,
			WindSpeed:  day.windSpeed,
			Humidity:   day.humidity,
		})
	}

	return result
}

func (p *Provider) fetchData(location string, isForecast bool, target interface{}) error {
	var body []byte
	var err error

	if p.useTestData {
		filename := "weather.weather.json"
		if isForecast {
			filename = "weather.forecast.json"
		}
		body, err = os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("error reading test file: %v", err)
		}
	} else {
		url := p.buildURL(location, isForecast)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("error making request: %v", err)
		}
		defer resp.Body.Close()

		if p.debugMode {
			fmt.Printf("Debug URL: %s\n", url)
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API error: %s", string(body))
		}
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	return nil
}

func (p *Provider) buildURL(location string, forecast bool) string {
	endpoint := "weather"
	if forecast {
		endpoint = "forecast"
	}

	if regexp.MustCompile(`^\d{5}$`).MatchString(location) {
		return fmt.Sprintf("http://api.openweathermap.org/data/2.5/%s?zip=%s,us&units=imperial&appid=%s",
			endpoint, location, p.apiKey)
	}
	return fmt.Sprintf("http://api.openweathermap.org/data/2.5/%s?q=%s,us&units=imperial&appid=%s",
		endpoint, url.QueryEscape(location), p.apiKey)
}

// Helper methods for processing forecast data...
