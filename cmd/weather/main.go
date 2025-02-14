package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/duluk/weather/pkg/weather"
	"github.com/duluk/weather/pkg/weather/openweather"
)

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

func displayCurrentWeather(w *weather.CurrentWeather) {
	header := fmt.Sprintf("Weather Summary for %s:", w.Location)
	fmt.Printf("%s\n", header)
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))
	fmt.Printf("Conditions:  %s\n", w.Conditions)
	fmt.Printf("Temperature: %.1f°F\n", w.Temperature)
	fmt.Printf("  Max:       %.1f°F\n", w.TempMax)
	fmt.Printf("  Min:       %.1f°F\n", w.TempMin)
	fmt.Printf("Feels Like:  %.1f°F\n", w.FeelsLike)
	fmt.Printf("Humidity:    %d%%\n", w.Humidity)
	fmt.Printf("Wind Speed:  %.1f mph\n", w.WindSpeed)
}

func displayForecast(f *weather.Forecast) {
	header := fmt.Sprintf("Weather Summary for %s:", f.Location)
	fmt.Printf("%s\n", header)
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))

	if f.Current != nil {
		displayCurrentWeather(f.Current)
		fmt.Println()
	}

	header = fmt.Sprintf("5-Day Forecast for %s:", f.Location)
	fmt.Printf("%s\n", header)
	fmt.Printf("%s\n", strings.Repeat("-", len(header)))

	for _, day := range f.DailyItems {
		fmt.Printf("%s: ", day.Date.Format("2006-01-02"))
		fmt.Printf("%-20s High: %4.1f°F. Low: %4.1f°F.",
			cases.Title(language.English).String(day.Conditions),
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

	apiKey, err := getAPIKey()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Please set the Open Weather API key, either via the environment variable, OPENWEATHER_API_KEY, or a file in ~/.config/weather/openweather_api_key")
		return
	}

	provider := openweather.New(apiKey, useTestData, debugMode)

	if wantForecast {
		forecast, err := provider.GetForecast(location)
		if err != nil {
			fmt.Printf("Error getting forecast: %v\n", err)
			return
		}
		displayForecast(forecast)
	} else {
		current, err := provider.GetCurrentWeather(location)
		if err != nil {
			fmt.Printf("Error getting current weather: %v\n", err)
			return
		}
		displayCurrentWeather(current)
	}
}
