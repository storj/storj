package main

import (
	"image/color"
	"io/ioutil"

	"github.com/loov/plot"
)

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

		// TODO: make independent of number of experiments

		{ // time plotting
			uploadTime := plot.NewDensity("s", asSeconds(m.Result("Upload").Durations))
			uploadTime.Stroke = color.NRGBA{0, 200, 0, 255}
			downloadTime := plot.NewDensity("s", asSeconds(m.Result("Download").Durations))
			downloadTime.Stroke = color.NRGBA{0, 0, 200, 255}
			deleteTime := plot.NewDensity("s", asSeconds(m.Result("Delete").Durations))
			deleteTime.Stroke = color.NRGBA{200, 0, 0, 255}

			flexTime := plot.NewHFlex()
			plots.Add(flexTime)
			flexTime.Add(70, plot.NewTextbox("time (s)"))
			flexTime.AddGroup(0,
				plot.NewGrid(),
				uploadTime,
				downloadTime,
				deleteTime,
				plot.NewTickLabels(),
			)
		}

		{ // speed plotting
			uploadSpeed := plot.NewDensity("MB/s", asSpeed(m.Result("Upload").Durations, m.Size.bytes))
			uploadSpeed.Stroke = color.NRGBA{0, 200, 0, 255}
			downloadSpeed := plot.NewDensity("MB/s", asSpeed(m.Result("Download").Durations, m.Size.bytes))
			downloadSpeed.Stroke = color.NRGBA{0, 0, 200, 255}

			flexSpeed := plot.NewHFlex()
			plots.Add(flexSpeed)

			speedGroup := plot.NewAxisGroup()
			speedGroup.X, speedGroup.Y = speed.X, speed.Y
			speedGroup.AddGroup(
				plot.NewGrid(),
				uploadSpeed,
				downloadSpeed,
				plot.NewTickLabels(),
			)

			flexSpeed.Add(70, plot.NewTextbox("speed (MB/s)"))
			flexSpeed.AddGroup(0, speedGroup)
		}
	}

	svgCanvas := plot.NewSVG(1500, 150*float64(len(measurements)))
	p.Draw(svgCanvas)

	return ioutil.WriteFile(*plotname, svgCanvas.Bytes(), 0755)
}
