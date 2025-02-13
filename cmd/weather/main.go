package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	Name string `json:"name"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run main.go <zipcode>")
		return
	}

	zipcode := os.Args[1]
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		apiKeyFile := os.ExpandEnv("$HOME/.config/weather/openweather_api_key")
		if _, err := os.Stat(apiKeyFile); err == nil {
			apiKeyBytes, err := os.ReadFile(apiKeyFile)
			if err != nil {
				fmt.Printf("Error reading API key file: %v\n", err)
				return
			}
			apiKey = string(apiKeyBytes)
		} else {
			fmt.Println("Please set the Open Weather API key, either via the environment variable, OPENWEATHER_API_KEY, or a file in ~/.config/weather/openweather_api_key")
			return
		}
	}

	url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?zip=%s, us&units=imperial&appid=%s",
		zipcode, apiKey)

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

	var weather WeatherData
	if err := json.Unmarshal(body, &weather); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return
	}

	fmt.Printf("\nWeather Summary for %s:\n", weather.Name)
	fmt.Printf("--------------------------------\n")
	if len(weather.Weather) > 0 {
		fmt.Printf("Conditions: %s\n", weather.Weather[0].Description)
	}
	fmt.Printf("Temperature: %.1f°F\n", weather.Main.Temp)
	fmt.Printf("Feels Like: %.1f°F\n", weather.Main.FeelsLike)
	fmt.Printf("Humidity: %d%%\n", weather.Main.Humidity)
}
