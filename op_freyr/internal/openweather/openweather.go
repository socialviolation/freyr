package openweather

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type LatLonTemp struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Temp int32   `json:"temp"`
}
type GeocodeResponse struct {
	Name       string            `json:"name"`
	LocalNames map[string]string `json:"local_names,omitempty"`
	Lat        float64           `json:"lat"`
	Lon        float64           `json:"lon"`
	Country    string            `json:"country"`
	State      string            `json:"state"`
}
type currentResponse struct {
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Timezone       string  `json:"timezone"`
	TimezoneOffset int     `json:"timezone_offset"`
	Current        struct {
		Dt         int     `json:"dt"`
		Sunrise    int     `json:"sunrise"`
		Sunset     int     `json:"sunset"`
		Temp       float64 `json:"temp"`
		FeelsLike  float64 `json:"feels_like"`
		Pressure   int     `json:"pressure"`
		Humidity   int     `json:"humidity"`
		DewPoint   float64 `json:"dew_point"`
		Uvi        float64 `json:"uvi"`
		Clouds     int     `json:"clouds"`
		Visibility int     `json:"visibility"`
		WindSpeed  float64 `json:"wind_speed"`
		WindDeg    int     `json:"wind_deg"`
		WindGust   float64 `json:"wind_gust"`
		Weather    []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
			Icon        string `json:"icon"`
		} `json:"weather"`
	} `json:"current"`
}

func GetLatLon(apikey, country, city string) (LatLonTemp, error) {
	url := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/direct?q=%s,%s&limit=5&appid=%s", city, country, apikey)
	resp, err := http.Get(url)
	if err != nil {
		return LatLonTemp{}, err
	}
	defer resp.Body.Close()

	var geocodeResponse []GeocodeResponse
	err = json.NewDecoder(resp.Body).Decode(&geocodeResponse)
	if err != nil {
		return LatLonTemp{}, err
	}
	return LatLonTemp{
		Lat: geocodeResponse[0].Lat,
		Lon: geocodeResponse[0].Lon,
	}, nil
}

func GetTempByCountry(apikey, country, city string) (LatLonTemp, error) {
	c, err := GetLatLon(apikey, country, city)
	if err != nil {
		return LatLonTemp{}, err
	}
	return GetTemp(apikey, c)
}

func GetTemp(apikey string, c LatLonTemp) (LatLonTemp, error) {
	url := fmt.Sprintf("https://api.openweathermap.org/data/3.0/onecall?lat=%f&lon=%f&exclude=hourly,daily&appid=%s", c.Lat, c.Lon, apikey)
	resp, err := http.Get(url)

	defer resp.Body.Close()
	var current currentResponse
	err = json.NewDecoder(resp.Body).Decode(&current)
	if err != nil {
		return LatLonTemp{}, err
	}
	c.Temp = int32(current.Current.Temp)
	return c, nil
}
