package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run main.go <zipcode>")
		return
	}

	zipcode := os.Args[1]
	if !regexp.MustCompile(`^\d{5}$`).MatchString(zipcode) {
		fmt.Println("Invalid zipcode format. Please provide a 5-digit US zipcode")
		return
	}

	apiKey, err := getAPIKey()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Please set the Open Weather API key, either via the environment variable, OPENWEATHER_API_KEY, or a file in ~/.config/weather/openweather_api_key")
		return
	}

	url := fmt.Sprintf("http://api.openweathermap.org/data/2.5/weather?zip=%s,us&units=imperial&appid=%s",
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: API returned status %d: %s\n", resp.StatusCode, string(body))
		return
	}

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
