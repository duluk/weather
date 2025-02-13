package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	netURL "net/url"
	"os"
	"regexp"
	"strings"
)

type WeatherData struct {
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
	Name string `json:"name"`
}

type ForecastData struct {
	List []struct {
		Dt   int64 `json:"dt"`
		Main struct {
			Temp      float64 `json:"temp"`
			FeelsLike float64 `json:"feels_like"`
			Humidity  int     `json:"humidity"`
		} `json:"main"`
		Weather []struct {
			Description string `json:"description"`
		} `json:"weather"`
		Wind struct {
			Speed float64 `json:"speed"`
		} `json:"wind"`
		DtTxt string `json:"dt_txt"`
	} `json:"list"`
	City struct {
		Name string `json:"name"`
	} `json:"city"`
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
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Println("Usage: go run main.go <zipcode or city,state> [forecast]")
		fmt.Println("Examples: go run main.go 02108")
		fmt.Println("          go run main.go \"Boston,MA\"")
		fmt.Println("          go run main.go \"Boston,MA\" forecast")
		return
	}

	location := os.Args[1]
	wantForecast := len(os.Args) == 3 && os.Args[2] == "forecast"

	apiKey, err := getAPIKey()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Please set the Open Weather API key, either via the environment variable, OPENWEATHER_API_KEY, or a file in ~/.config/weather/openweather_api_key")
		return
	}

	var url string
	if regexp.MustCompile(`^\d{5}$`).MatchString(location) {
		// It's a zipcode
		if wantForecast {
			url = fmt.Sprintf("http://api.openweathermap.org/data/2.5/forecast?zip=%s,us&units=imperial&appid=%s",
				location, apiKey)
		} else {
			url = fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?zip=%s,us&units=imperial&appid=%s",
				location, apiKey)
		}
	} else if regexp.MustCompile(`^[a-zA-Z]+, ?[A-Z]{2}$`).MatchString(location) {
		// It's a city name
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error: API returned status %d: %s\n", resp.StatusCode, string(body))
		return
	}

	if wantForecast {
		var forecast ForecastData
		if err := json.Unmarshal(body, &forecast); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			return
		}

		header := fmt.Sprintf("Weather Summary for %s:", forecast.City.Name)
		fmt.Printf("%s\n", header)
		fmt.Printf("%s\n", strings.Repeat("-", len(header)))

		// Show current conditions from first forecast item
		if len(forecast.List) > 0 && len(forecast.List[0].Weather) > 0 {
			fmt.Printf("Current Conditions: %s\n", forecast.List[0].Weather[0].Description)
			fmt.Printf("Temperature: %.1f°F\n", forecast.List[0].Main.Temp)
			fmt.Printf("Feels Like: %.1f°F\n", forecast.List[0].Main.FeelsLike)
			fmt.Printf("Humidity: %d%%\n", forecast.List[0].Main.Humidity)
			fmt.Printf("Wind Speed: %.1f mph\n\n", forecast.List[0].Wind.Speed)
		}

		header = fmt.Sprintf("5-Day Forecast for %s:", forecast.City.Name)
		fmt.Printf("%s\n", header)
		fmt.Printf("%s\n", strings.Repeat("-", len(header)))

		// Group forecasts by day to show one forecast per day
		var lastDate string
		for _, item := range forecast.List {
			date := strings.Split(item.DtTxt, " ")[0]
			time := strings.Split(item.DtTxt, " ")[1]

			// Only show the noon forecast for each day
			if time == "12:00:00" {
				if date != lastDate {
					fmt.Printf("\n%s: ", date)
					if len(item.Weather) > 0 {
						fmt.Printf("%s with a temp of %.1f°F ",
							strings.Title(item.Weather[0].Description),
							item.Main.Temp)
					}
					fmt.Printf("(real feel %.1f°F) ", item.Main.FeelsLike)
					if item.Wind.Speed > 0 {
						fmt.Printf("Winds up to %.1f mph", item.Wind.Speed)
					}
					if item.Main.Humidity > 0 {
						fmt.Printf(" and %d%% humidity", item.Main.Humidity)
					}
					lastDate = date
				}
			}
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
			fmt.Printf("Conditions: %s\n", weather.Weather[0].Description)
		}
		fmt.Printf("Temperature: %.1f°F\n", weather.Main.Temp)
		fmt.Printf("Feels Like: %.1f°F\n", weather.Main.FeelsLike)
		fmt.Printf("Humidity: %d%%\n", weather.Main.Humidity)
		fmt.Printf("Wind Speed: %.1f mph\n", weather.Wind.Speed)
	}
}
