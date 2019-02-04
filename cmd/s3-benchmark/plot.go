// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"image/color"
	"io/ioutil"
	"time"

	"github.com/loov/plot"
)

var palette = []color.Color{
	color.NRGBA{0, 200, 0, 255},
	color.NRGBA{0, 0, 200, 255},
	color.NRGBA{200, 0, 0, 255},
}

// Plot plots measurements into filename as an svg
func Plot(filename string, measurements []Measurement) error {
	p := plot.New()
	p.X.Min = 0
	p.X.Max = 10
	p.X.MajorTicks = 10
	p.X.MinorTicks = 10

	speed := plot.NewAxisGroup()
	speed.Y.Min = 0
	speed.Y.Max = 1
	speed.X.Min = 0
	speed.X.Max = 30
	speed.X.MajorTicks = 10
	speed.X.MinorTicks = 10

	rows := plot.NewVStack()
	rows.Margin = plot.R(5, 5, 5, 5)
	p.Add(rows)

	for _, m := range measurements {
		row := plot.NewHFlex()
		rows.Add(row)
		row.Add(35, plot.NewTextbox(m.Size.String()))

		plots := plot.NewVStack()
		row.Add(0, plots)

		{ // time plotting
			group := []plot.Element{plot.NewGrid()}

			for i, result := range m.Results {
				time := plot.NewDensity("s", asSeconds(result.Durations))
				time.Stroke = palette[i%len(palette)]
				group = append(group, time)
			}

			group = append(group, plot.NewTickLabels())

			flexTime := plot.NewHFlex()
			plots.Add(flexTime)
			flexTime.Add(70, plot.NewTextbox("time (s)"))
			flexTime.AddGroup(0, group...)
		}

		{ // speed plotting
			group := []plot.Element{plot.NewGrid()}

			for i, result := range m.Results {
				if !result.WithSpeed {
					continue
				}

				speed := plot.NewDensity("MB/s", asSpeed(result.Durations, m.Size.Int64()))
				speed.Stroke = palette[i%len(palette)]
			}

			group = append(group, plot.NewTickLabels())

			flexSpeed := plot.NewHFlex()
			plots.Add(flexSpeed)

			speedGroup := plot.NewAxisGroup()
			speedGroup.X, speedGroup.Y = speed.X, speed.Y
			speedGroup.AddGroup(group...)

			flexSpeed.Add(70, plot.NewTextbox("speed (MB/s)"))
			flexSpeed.AddGroup(0, speedGroup)
		}
	}

	svgCanvas := plot.NewSVG(1500, 150*float64(len(measurements)))
	p.Draw(svgCanvas)

	return ioutil.WriteFile(filename, svgCanvas.Bytes(), 0755)
}

func asSeconds(durations []time.Duration) []float64 {
	xs := make([]float64, 0, len(durations))
	for _, dur := range durations {
		xs = append(xs, dur.Seconds())
	}
	return xs
}

func asSpeed(durations []time.Duration, size int64) []float64 {
	const MB = 1 << 20
	xs := make([]float64, 0, len(durations))
	for _, dur := range durations {
		xs = append(xs, (float64(size)/MB)/dur.Seconds())
	}
	return xs
}
