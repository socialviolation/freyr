package trig

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

type Args struct {
	Duration string
	Min      int32
	Max      int32
	Current  int32
}

func makeCanvas(cols, rows int) [][]string {
	matrix := make([][]string, rows)
	for i := range matrix {
		matrix[i] = make([]string, cols)
	}
	return matrix
}

func RenderValues(a Args) string {
	duration, err := time.ParseDuration(a.Duration)
	if err != nil {
		return "could not render chart"
	}
	_, secondsSinceIncrement := getStart(duration)
	dSeconds := duration.Seconds()
	results := make([]float64, int(dSeconds))
	var yMin, yMax float64

	for i := 1; i <= int(dSeconds)-1; i++ {
		args := Args{
			Duration: a.Duration,
			Min:      a.Min,
			Max:      a.Max,
			Current:  int32(i),
		}
		value, err := GetValue(args)
		if err != nil {
			fmt.Println(err)
			return "could not render chart"
		}
		results[i] = value
		if value < yMin {
			yMin = value
		}
		if value > yMax {
			yMax = value
		}
	}

	canvasAxisPadding := len(strconv.Itoa(int(a.Max))) + 1
	canvasAxisOffset := 1
	canvasWidth := 120 + canvasAxisOffset
	canvasHeight := 12

	canvas := makeCanvas(canvasWidth, canvasHeight)
	translatedSeconds := int(translate(float64(secondsSinceIncrement), 0, dSeconds, 0, float64(canvasWidth-1)))
	for row := range canvas {
		for col := range canvas[row] {
			if col < canvasAxisOffset {
				currentTransNode := canvasHeight - row
				nodeY := int(translate(float64(currentTransNode), 0, float64(canvasHeight-1), float64(a.Min), float64(a.Max))) - 1
				canvas[row][col] = fmt.Sprintf("%-*d", canvasAxisPadding, nodeY)
				continue
			}

			if col == translatedSeconds+canvasAxisOffset {
				canvas[row][col] = "|"
			} else {
				canvas[row][col] = " "
			}
		}
	}

	sampledResults := make([]float64, canvasWidth-canvasAxisOffset)
	for i := 0; i < len(sampledResults)-1; i++ {
		transI := int(translate(float64(i), 0, float64(canvasWidth-1), 0, dSeconds-1))
		sampledResults[i] = results[transI]
	}

	for i, value := range sampledResults {
		constrainedY := int(translate(value, yMin, yMax, 0, float64(canvasHeight-1)))
		if i == translatedSeconds {
			canvas[constrainedY][i+canvasAxisOffset] = "#"
		} else {
			canvas[constrainedY][i+canvasAxisOffset] = "*"
		}
	}

	// Print the canvas
	var output string
	for y := range canvas {
		for x := range canvas[y] {
			output += fmt.Sprintf(canvas[y][x])
		}
		output += fmt.Sprintf("\n")
	}
	return output
}

func GetValue(a Args) (float64, error) {
	d, err := time.ParseDuration(a.Duration)
	if err != nil {
		return 0, err
	}

	if a.Current == 0 {
		_, secondsSinceIncrement := getStart(d)
		a.Current = int32(secondsSinceIncrement)
	}

	angle := (2 * math.Pi * (float64(a.Current))) / d.Seconds()
	return math.Sin(angle), nil
}

func getStart(d time.Duration) (time.Time, int) {
	now := time.Now()
	s := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	elapsed := now.Sub(s)
	numDurations := int(elapsed / d)
	mostRecentIncrement := s.Add(time.Duration(numDurations) * d)
	secondsSinceIncrement := int(now.Sub(mostRecentIncrement).Seconds())
	return mostRecentIncrement, secondsSinceIncrement
}

func translate(x, inMin, inMax, outMin, outMax float64) float64 {
	proportion := (x - inMin) / (inMax - inMin)
	return outMin + proportion*(outMax-outMin)
}
