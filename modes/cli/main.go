package main

import (
	"fmt"
	"github.com/socialviolation/freyr/modes/openweather"
	"github.com/socialviolation/freyr/modes/trig"
	"os"
)

func testWeather() {
	l := openweather.Location{
		Country: "AU",
		City:    "Melbourne",
	}
	temp, err := openweather.GetTempByCountry(os.Getenv("OWK"), l)
	if err != nil {
		panic(err)
	}

	fmt.Println(temp)
}

func testTrig() {
	res := trig.RenderValues(trig.Args{
		Duration: "120s",
		Min:      5,
		Max:      20,
	})
	fmt.Println(res)
}

func main() {
	testTrig()
}
