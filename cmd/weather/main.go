package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	netURL "net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

func getAPIKey() (string, error) {
	if apiKey := os.Getenv("OPENWEATHER_API_KEY"); apiKey != "" {
		return apiKey, nil
	}

	apiKeyFile := os.ExpandEnv("$HOME/.config/weather/openweather_api_key")
	if _, err := os.Stat(apiKeyFile); err == nil {
		apiKeyBytes, err := os.ReadFile(apiKeyFile)
		if err != nil {
			return "", fmt.Errorf("error reading API key file: %v", err)
		}
		return strings.TrimSpace(string(apiKeyBytes)), nil
	}

	return "", fmt.Errorf("API key not found in environment or config file")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: weather <zipcode or city,state> [forecast] [-test] [-debug]")
		fmt.Println("Examples: weather 02108")
		fmt.Println("          weather \"Boston,MA\"")
		fmt.Println("          weather \"Boston,MA\" forecast")
		fmt.Println("          weather \"Boston,MA\" forecast -test")
		return
	}

	location := os.Args[1]
	wantForecast := false
	useTestData := false
	debugMode := false

	// Parse flags
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "forecast":
			wantForecast = true
		case "-test":
			useTestData = true
		case "-debug":
			debugMode = true
		}
	}

	var body []byte
	var err error

	if useTestData {
		// Use local test files
		filename := "weather.weather.json"
		if wantForecast {
			filename = "weather.forecast.json"
		}
		body, err = os.ReadFile(filename)
		if err != nil {
			fmt.Printf("Error reading test file %s: %v\n", filename, err)
			return
		}
	} else {
		// Original API call code
		apiKey, err := getAPIKey()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("Please set the Open Weather API key, either via the environment variable, OPENWEATHER_API_KEY, or a file in ~/.config/weather/openweather_api_key")
			return
		}

		var url string
		if regexp.MustCompile(`^\d{5}$`).MatchString(location) {
			if wantForecast {
				url = fmt.Sprintf("http://api.openweathermap.org/data/2.5/forecast?zip=%s,us&units=imperial&appid=%s",
					location, apiKey)
			} else {
				url = fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?zip=%s,us&units=imperial&appid=%s",
					location, apiKey)
			}
		} else if regexp.MustCompile(`^[a-zA-Z]+, ?[A-Z]{2}$`).MatchString(location) {
			location = strings.Replace(location, ", ", ",", 1)
			if wantForecast {
				url = fmt.Sprintf("http://api.openweathermap.org/data/2.5/forecast?q=%s,us&units=imperial&appid=%s",
					netURL.QueryEscape(location), apiKey)
			} else {
				url = fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?q=%s,us&units=imperial&appid=%s",
					netURL.QueryEscape(location), apiKey)
			}
		} else {
			fmt.Println("Invalid location format. Please provide a zipcode or city,state")
			return
		}

		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error making request: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if debugMode {
			fmt.Printf("Debug raw response: %s\n", resp.Body)
		}

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			return
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Error: API returned status %d: %s\n", resp.StatusCode, string(body))
			return
		}
	}

	if wantForecast {
		var forecast ForecastData
		if err := json.Unmarshal(body, &forecast); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			return
		}

		// if debugMode {
		// 	fmt.Printf("Debug raw forecast: %+v\n", forecast)
		// }

		header := fmt.Sprintf("Weather Summary for %s:", forecast.City.Name)
		fmt.Printf("%s\n", header)
		fmt.Printf("%s\n", strings.Repeat("-", len(header)))

		if len(forecast.List) > 0 && len(forecast.List[0].Weather) > 0 {
			fmt.Printf("Current:     %s\n", forecast.List[0].Weather[0].Description)
			fmt.Printf("Temperature: %.1f°F\n", forecast.List[0].Main.Temp)
			fmt.Printf("  Max:       %.1f°F\n", forecast.List[0].Main.TempMax)
			fmt.Printf("  Min:       %.1f°F\n", forecast.List[0].Main.TempMin)
			fmt.Printf("Feels Like:  %.1f°F\n", forecast.List[0].Main.FeelsLike)
			fmt.Printf("Humidity:    %d%%\n", forecast.List[0].Main.Humidity)
			fmt.Printf("Wind Speed:  %.1f mph\n\n", forecast.List[0].Wind.Speed)
		}

		header = fmt.Sprintf("5-Day Forecast for %s:", forecast.City.Name)
		fmt.Printf("%s\n", header)
		fmt.Printf("%s\n", strings.Repeat("-", len(header)))

		type DayForecast struct {
			High        float64
			Low         float64
			Description string
			WindSpeed   float64
			Humidity    int
		}
		dailyForecasts := make(map[string]*DayForecast)

		for _, item := range forecast.List {
			date := strings.Split(item.DateText, " ")[0]
			time := strings.Split(item.DateText, " ")[1]

			// Only process readings between 06:00 and 00:00
			hour := strings.Split(time, ":")[0]
			if hour < "06" {
				continue
			}

			if _, exists := dailyForecasts[date]; !exists {
				dailyForecasts[date] = &DayForecast{
					High:        -1000,
					Low:         1000,
					Description: item.Weather[0].Description,
					WindSpeed:   0,
					Humidity:    item.Main.Humidity,
				}
			}

			day := dailyForecasts[date]
			// Use the provided max/min temperatures
			if item.Main.TempMax > day.High {
				day.High = item.Main.TempMax
			}
			if item.Main.TempMin < day.Low {
				day.Low = item.Main.TempMin
			}
			// Track highest wind speed from any interval
			if item.Wind.Speed > day.WindSpeed {
				day.WindSpeed = item.Wind.Speed
			}

			// Still use noon for the general description and humidity
			if strings.Contains(item.DateText, "12:00:00") {
				day.Description = item.Weather[0].Description
				day.Humidity = item.Main.Humidity
			}
		}

		// Output the daily forecasts in order
		var dates []string
		for date := range dailyForecasts {
			dates = append(dates, date)
		}
		sort.Strings(dates)

		for _, date := range dates {
			day := dailyForecasts[date]
			fmt.Printf("%s: ", date)
			fmt.Printf("%-20s High: %4.1f°F. Low: %4.1f°F.",
				cases.Title(language.English).String(day.Description),
				day.High,
				day.Low)
			if day.WindSpeed > 0 {
				fmt.Printf(" Max winds: %4.1f mph.", day.WindSpeed)
			}
			if day.Humidity > 0 {
				fmt.Printf(" Humidity: %d%%.", day.Humidity)
			}
			fmt.Println()
		}
	} else {
		var weather WeatherData
		if err := json.Unmarshal(body, &weather); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			return
		}

		header := fmt.Sprintf("Weather Summary for %s:", weather.Name)
		fmt.Printf("%s\n", header)
		fmt.Printf("%s\n", strings.Repeat("-", len(header)))
		if len(weather.Weather) > 0 {
			fmt.Printf("Conditions:  %s\n", weather.Weather[0].Description)
		}
		fmt.Printf("Temperature: %.1f°F\n", weather.Main.Temp)
		fmt.Printf("  Max:       %.1f°F\n", weather.Main.TempMax)
		fmt.Printf("  Min:       %.1f°F\n", weather.Main.TempMin)
		fmt.Printf("Feels Like:  %.1f°F\n", weather.Main.FeelsLike)
		fmt.Printf("Humidity:    %d%%\n", weather.Main.Humidity)
		fmt.Printf("Wind Speed:  %.1f mph\n", weather.Wind.Speed)
	}
}
