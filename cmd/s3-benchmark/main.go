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
	count := flag.Int("count", 10, "run each benchmark n times")
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
	log.Println("creating bucket", bucket)
	err = client.MakeBucket(bucket, *location)
	if err != nil {
		log.Fatal("failed to create bucket: ", bucket, ": ", err)
	}

	defer func() {
		log.Println("removing bucket")
		err := client.RemoveBucket(bucket)
		if err != nil {
			log.Fatal("failed to remove bucket: ", bucket)
		}
	}()

	log.Println("Benchmarking:")
	measurements := []Measurement{}
	for _, size := range sizes.Sizes() {
		measurement, err := Benchmark(client, bucket, size, *count)
		if err != nil {
			log.Fatal(err)
		}
		measurements = append(measurements, measurement)
	}

	fmt.Print("\n\n")
	for _, m := range measurements {
		m.PrintHistograms()
	}

	if *plotname != "" {
		p := plot.New()
		p.X.Min = 0
		p.X.Max = 10
		p.X.MajorTicks = 10
		p.X.MinorTicks = 10

		stack := plot.NewVStack()
		stack.Margin = plot.R(5, 5, 5, 5)
		p.Add(stack)

		for _, m := range measurements {
			upload := plot.NewDensity("s", asSeconds(m.Upload))
			upload.Stroke = color.NRGBA{0, 200, 0, 255}
			download := plot.NewDensity("s", asSeconds(m.Download))
			download.Stroke = color.NRGBA{0, 0, 200, 255}
			delete := plot.NewDensity("s", asSeconds(m.Delete))
			delete.Stroke = color.NRGBA{200, 0, 0, 255}

			flex := plot.NewHFlex()
			flex.Add(100, plot.NewTextbox(m.Size.String()))
			flex.AddGroup(0,
				plot.NewGrid(),
				upload,
				download,
				delete,
				plot.NewTickLabels(),
			)
			stack.Add(flex)
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

type Measurement struct {
	Size     Size
	Upload   []time.Duration
	Download []time.Duration
	Delete   []time.Duration
}

func (m *Measurement) PrintHistograms() {
	const binCount = 10

	fmt.Println("Upload", m.Size)
	upload := hrtime.NewDurationHistogram(m.Upload, binCount)
	upload.WriteStatsTo(os.Stdout)

	fmt.Println("Download", m.Size)
	download := hrtime.NewDurationHistogram(m.Download, binCount)
	download.WriteStatsTo(os.Stdout)

	fmt.Println("Delete", m.Size)
	delete := hrtime.NewDurationHistogram(m.Delete, binCount)
	delete.WriteStatsTo(os.Stdout)
}

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
