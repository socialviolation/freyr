package modes

type OperatorSpec struct {
	Mode    string      `json:"mode,omitempty"`
	Weather WeatherMode `json:"weather,omitempty"`
	Trig    TrigMode    `json:"trig,omitempty"`
}

type WeatherMode struct {
	Country string `json:"country,omitempty"`
	City    string `json:"city,omitempty"`
	APIKey  string `json:"apiKey,omitempty"`
}

type TrigMode struct {
	Duration string `json:"period,omitempty"`
	Min      int32  `json:"min,omitempty"`
	Max      int32  `json:"max,omitempty"`
}
