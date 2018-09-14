// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"text/tabwriter"
	"time"

	"github.com/loov/hrtime"
	"github.com/loov/plot"
	minio "github.com/minio/minio-go"
)

func main() {
	endpoint := flag.String("endpoint", "127.0.0.1:7777", "endpoint address")
	accesskey := flag.String("accesskey", "insecure-dev-access-key", "access key")
	secretkey := flag.String("secretkey", "insecure-dev-secret-key", "secret key")
	useSSL := flag.Bool("use-ssl", true, "use ssl")
	location := flag.String("location", "", "bucket location")
	count := flag.Int("count", 50, "run each benchmark n times")
	plotname := flag.String("plot", "", "plot results")

	sizes := &Sizes{
		Default: []Size{{1 << 10}, {256 << 10}, {1 << 20}, {32 << 20}, {63 << 20}},
	}
	flag.Var(sizes, "size", "sizes to test with")

	flag.Parse()

	client, err := minio.New(*endpoint, *accesskey, *secretkey, *useSSL)
	if err != nil {
		log.Fatal(err)
	}

	bucket := time.Now().Format("bucket-2006-01-02-150405")
	log.Println("Creating bucket", bucket)
	err = client.MakeBucket(bucket, *location)
	if err != nil {
		log.Fatal("failed to create bucket: ", bucket, ": ", err)
	}

	defer func() {
		log.Println("Removing bucket")
		err := client.RemoveBucket(bucket)
		if err != nil {
			log.Fatal("failed to remove bucket: ", bucket)
		}
	}()

	measurements := []Measurement{}
	for _, size := range sizes.Sizes() {
		measurement, err := Benchmark(client, bucket, size, *count)
		if err != nil {
			log.Fatal(err)
		}
		measurements = append(measurements, measurement)
	}

	fmt.Print("\n\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
		"Size", "",
		"Avg", "",
		"Max", "",
		"P50", "", "P90", "", "P99", "",
	)
	fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
		"", "",
		"s", "MB/s",
		"s", "MB/s",
		"s", "MB/s", "s", "MB/s", "s", "MB/s",
	)
	for _, m := range measurements {
		m.PrintStats(w)
	}
	_ = w.Flush()

	if *plotname != "" {
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
				uploadTime := plot.NewDensity("s", asSeconds(m.Upload))
				uploadTime.Stroke = color.NRGBA{0, 200, 0, 255}
				downloadTime := plot.NewDensity("s", asSeconds(m.Download))
				downloadTime.Stroke = color.NRGBA{0, 0, 200, 255}
				deleteTime := plot.NewDensity("s", asSeconds(m.Delete))
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
				uploadSpeed := plot.NewDensity("MB/s", asSpeed(m.Upload, m.Size.bytes))
				uploadSpeed.Stroke = color.NRGBA{0, 200, 0, 255}
				downloadSpeed := plot.NewDensity("MB/s", asSpeed(m.Download, m.Size.bytes))
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

		svgcanvas := plot.NewSVG(1500, 150*float64(len(measurements)))
		p.Draw(svgcanvas)

		err := ioutil.WriteFile(*plotname, svgcanvas.Bytes(), 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
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

// Measurement contains measurements for different requests
type Measurement struct {
	Size     Size
	Upload   []time.Duration
	Download []time.Duration
	Delete   []time.Duration
}

// PrintStats prints important valueas about the measurement
func (m *Measurement) PrintStats(w io.Writer) {
	const binCount = 10

	upload := hrtime.NewDurationHistogram(m.Upload, binCount)
	download := hrtime.NewDurationHistogram(m.Download, binCount)
	delete := hrtime.NewDurationHistogram(m.Delete, binCount)

	hists := []struct {
		L string
		H *hrtime.Histogram
	}{
		{"Upload", upload},
		{"Download", download},
		{"Delete", delete},
	}

	sec := func(ns float64) string {
		return fmt.Sprintf("%.2f", ns/1e9)
	}
	speed := func(ns float64) string {
		return fmt.Sprintf("%.2f", (float64(m.Size.bytes)/(1<<20))/(ns/1e9))
	}

	for _, hist := range hists {
		if hist.L == "Delete" {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				m.Size, hist.L,
				sec(hist.H.Average), "",
				sec(hist.H.Maximum), "",
				sec(hist.H.P50), "",
				sec(hist.H.P90), "",
				sec(hist.H.P99), "",
			)
			continue
		}
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			m.Size, hist.L,
			sec(hist.H.Average), speed(hist.H.Average),
			sec(hist.H.Maximum), speed(hist.H.Maximum),
			sec(hist.H.P50), speed(hist.H.P50),
			sec(hist.H.P90), speed(hist.H.P90),
			sec(hist.H.P99), speed(hist.H.P99),
		)
	}
}

// Benchmark runs benchmarks on bucket with given size
func Benchmark(client *minio.Client, bucket string, size Size, count int) (Measurement, error) {
	log.Print("Benchmarking size ", size.String(), " ")

	data := make([]byte, size.bytes)
	result := make([]byte, size.bytes)

	defer fmt.Println()

	measurement := Measurement{}
	measurement.Size = size
	for k := 0; k < count; k++ {
		fmt.Print(".")

		rand.Read(data[:])
		{ // uploading
			start := hrtime.Now()
			_, err := client.PutObject(bucket, "data", bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
				ContentType: "application/octet-stream",
			})
			finish := hrtime.Now()
			if err != nil {
				return measurement, fmt.Errorf("upload failed: %v", err)
			}
			measurement.Upload = append(measurement.Upload, (finish - start))
		}

		{ // downloading
			start := hrtime.Now()
			reader, err := client.GetObject(bucket, "data", minio.GetObjectOptions{})
			if err != nil {
				return measurement, fmt.Errorf("get object failed: %v", err)
			}

			var n int
			n, err = reader.Read(result)
			if err != nil && err != io.EOF {
				return measurement, fmt.Errorf("download failed: %v", err)
			}
			finish := hrtime.Now()

			if !bytes.Equal(data, result[:n]) {
				return measurement, errors.New("upload/download do not match")
			}

			measurement.Download = append(measurement.Download, (finish - start))
		}

		{ // deleting
			start := hrtime.Now()
			err := client.RemoveObject(bucket, "data")
			if err != nil {
				return measurement, fmt.Errorf("delete failed: %v", err)
			}
			finish := hrtime.Now()
			measurement.Delete = append(measurement.Delete, (finish - start))
		}
	}

	return measurement, nil
}
