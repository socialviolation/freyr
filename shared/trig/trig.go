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

func GetValue(a Args) (float64, error) {
	angle, err := calculateAngle(a)
	if err != nil {
		return 0, err
	}

	t := translate(angle, -1, 1, float64(a.Min), float64(a.Max))
	return t, nil
}

func RenderChart(a Args) string {
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
				nodeY := int(translate(float64(row), 0, float64(canvasHeight-1), float64(a.Min), float64(a.Max))) - 1
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
	for yI := len(canvas) - 1; yI >= 0; yI-- {
		for x := range canvas[yI] {
			output += fmt.Sprintf(canvas[yI][x])
		}
		output += fmt.Sprintf("\n")
	}
	return output
}

func calculateAngle(a Args) (float64, error) {
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
	now := time.Now().UTC()
	elapsedSeconds := int(now.Unix() % 86400)
	startOfDay := now.Add(-time.Duration(elapsedSeconds) * time.Second)
	numDurations := int(elapsedSeconds) / int(d.Seconds())
	mostRecentIncrement := startOfDay.Add(time.Duration(numDurations) * d)
	secondsSinceIncrement := int(now.Sub(mostRecentIncrement).Seconds())
	return mostRecentIncrement, secondsSinceIncrement
}

func translate(x, inMin, inMax, outMin, outMax float64) float64 {
	proportion := (x - inMin) / (inMax - inMin)
	return outMin + proportion*(outMax-outMin)
}
