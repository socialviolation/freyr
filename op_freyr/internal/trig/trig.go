package trig

import (
	"math"
	"time"
)

func GetValue(seconds float64, min, max int32) int32 {
	// Get the current time
	currentTime := time.Now()

	// Calculate the elapsed time in seconds since the start of the hour
	elapsedSeconds := float64(currentTime.Minute()*60 + currentTime.Second())

	// Define the period of the function (in seconds)
	period := seconds // 1800 seconds = 30 minutes

	// Calculate the angle (in radians) based on the elapsed time and period
	angle := 2 * math.Pi * elapsedSeconds / period

	// Calculate the trigonometric function (you can use sine, cosine, or any other trigonometric function)
	value := math.Sin(angle)

	// Map the trigonometric value to the desired range
	return int32(math.Min(math.Max((value+1)*0.5*float64(max)+1, float64(min)), float64(max)))
}
