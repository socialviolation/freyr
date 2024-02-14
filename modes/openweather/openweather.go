package openweather

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Location struct {
	Country string `json:"country"`
	City    string `json:"city"`
}

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
	Coord struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Base string `json:"base"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		Pressure  int     `json:"pressure"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Visibility int `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   int     `json:"deg"`
	} `json:"wind"`
	Clouds struct {
		All int `json:"all"`
	} `json:"clouds"`
	Dt  int `json:"dt"`
	Sys struct {
		Type    int    `json:"type"`
		ID      int    `json:"id"`
		Country string `json:"country"`
		Sunrise int    `json:"sunrise"`
		Sunset  int    `json:"sunset"`
	} `json:"sys"`
	Timezone int    `json:"timezone"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Cod      int    `json:"cod"`
}

func GetLatLon(apikey string, l Location) (LatLonTemp, error) {
	url := fmt.Sprintf("http://api.openweathermap.org/geo/1.0/direct?q=%s,%s&limit=5&appid=%s", l.City, l.Country, apikey)
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

func GetTempByCountry(apikey string, l Location) (LatLonTemp, error) {
	c, err := GetLatLon(apikey, l)
	if err != nil {
		return LatLonTemp{}, err
	}
	return GetTemp(apikey, c)
}

func GetTemp(apikey string, c LatLonTemp) (LatLonTemp, error) {
	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&exclude=hourly,daily&appid=%s&units=metric", c.Lat, c.Lon, apikey)
	resp, err := http.Get(url)
	if err != nil {
		return LatLonTemp{}, err
	}

	defer resp.Body.Close()
	var current currentResponse
	err = json.NewDecoder(resp.Body).Decode(&current)
	if err != nil {
		return LatLonTemp{}, err
	}
	c.Temp = int32(current.Main.Temp)
	return c, nil
}
