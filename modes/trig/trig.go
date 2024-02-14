package trig

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

type Args struct {
	Duration string
	Start    time.Time
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
	start, secondsSinceIncrement := getStart(a.Start, duration)
	dSeconds := duration.Seconds()
	results := make([]float64, int(dSeconds))
	var yMin, yMax float64

	for i := 1; i <= int(dSeconds)-1; i++ {
		args := Args{
			Duration: a.Duration,
			Start:    start,
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
				nodeY := int(translate(float64(currentTransNode), 0, float64(canvasHeight-1), float64(a.Min), float64(a.Max)))
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
		_, secondsSinceIncrement := getStart(a.Start, d)
		a.Current = int32(secondsSinceIncrement)
	}

	angle := (2 * math.Pi * (float64(a.Current))) / d.Seconds()
	return math.Sin(angle), nil
}

func getStart(s time.Time, d time.Duration) (time.Time, int) {
	now := time.Now()
	if (s == time.Time{}) {
		s = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	elapsed := now.Sub(s)
	numDurations := int(elapsed / d)
	mostRecentIncrement := s.Add(time.Duration(numDurations) * d)
	secondsSinceIncrement := int(now.Sub(mostRecentIncrement).Seconds())
	//fmt.Printf("Number of time durations passed: %d\n", numDurations)
	//fmt.Printf("Seconds since the most recent increment: %d\n", secondsSinceIncrement)

	return mostRecentIncrement, secondsSinceIncrement
}

func translate(x, inMin, inMax, outMin, outMax float64) float64 {
	proportion := (x - inMin) / (inMax - inMin)
	return outMin + proportion*(outMax-outMin)
}

func constrain(value, min, max float64) int {
	rangeValue := max - min
	numIndexes := int(math.Ceil(rangeValue))

	stepSize := rangeValue / float64(numIndexes)
	index := int((value - min) / stepSize)
	if index < 0 {
		index = 0
	}
	if index >= numIndexes {
		index = numIndexes - 1
	}
	return index
}
