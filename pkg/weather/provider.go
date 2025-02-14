package weather

import "time"

type Provider interface {
	GetCurrentWeather(location string) (*CurrentWeather, error)
	GetForecast(location string) (*Forecast, error)
}

type CurrentWeather struct {
	Location    string
	Conditions  string
	Temperature float64
	FeelsLike   float64
	TempMax     float64
	TempMin     float64
	Humidity    int
	WindSpeed   float64
}

type DailyForecast struct {
	Date       time.Time
	Conditions string
	High       float64
	Low        float64
	WindSpeed  float64
	Humidity   int
}

type Forecast struct {
	Location   string
	Current    *CurrentWeather
	DailyItems []DailyForecast
}
