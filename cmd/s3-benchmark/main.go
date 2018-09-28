// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/loov/hrtime"

	"storj.io/storj/internal/memory"
)

func main() {
	var conf Config

	flag.StringVar(&conf.Endpoint, "endpoint", "127.0.0.1:7777", "endpoint address")
	flag.StringVar(&conf.AccessKey, "accesskey", "insecure-dev-access-key", "access key")
	flag.StringVar(&conf.SecretKey, "secretkey", "insecure-dev-secret-key", "secret key")
	flag.BoolVar(&conf.NoSSL, "no-ssl", false, "disable ssl")

	clientName := flag.String("client", "minio", "client to use for requests (supported: minio, aws-cli)")

	location := flag.String("location", "", "bucket location")
	count := flag.Int("count", 50, "benchmark count")
	duration := flag.Duration("time", 2*time.Minute, "maximum benchmark time per size")

	suffix := time.Now().Format("-2006-01-02-150405")

	plotname := flag.String("plot", "plot"+suffix+".svg", "plot results")

	sizes := &memory.Sizes{
		Default: []memory.Size{
			1 * memory.KB,
			256 * memory.KB,
			1 * memory.MB,
			32 * memory.MB,
			64 * memory.MB,
			256 * memory.MB,
		},
	}
	flag.Var(sizes, "size", "sizes to test with")

	flag.Parse()

	var client Client
	var err error

	switch *clientName {
	default:
		log.Println("unknown client name ", *clientName, " defaulting to minio")
		fallthrough
	case "minio":
		client, err = NewMinio(conf)
	case "aws-cli":
		client, err = NewAWSCLI(conf)
	}
	if err != nil {
		log.Fatal(err)
	}

	bucket := "bucket" + suffix
	log.Println("Creating bucket", bucket)
	err = client.MakeBucket(bucket, *location)
	if err != nil {
		log.Fatalf("failed to create bucket %q: %+v\n", bucket, err)
	}

	defer func() {
		log.Println("Removing bucket")
		err := client.RemoveBucket(bucket)
		if err != nil {
			log.Fatalf("failed to remove bucket %q", bucket)
		}
	}()

	measurements := []Measurement{}
	for _, size := range sizes.Sizes() {
		measurement, err := Benchmark(client, bucket, size, *count, *duration)
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
		err := Plot(*plotname, measurements)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// Measurement contains measurements for different requests
type Measurement struct {
	Size    memory.Size
	Results []*Result
}

// Result contains durations for specific tests
type Result struct {
	Name      string
	WithSpeed bool
	Durations []time.Duration
}

// Result finds or creates a result with the specified name
func (m *Measurement) Result(name string) *Result {
	for _, x := range m.Results {
		if x.Name == name {
			return x
		}
	}

	r := &Result{}
	r.Name = name
	m.Results = append(m.Results, r)
	return r
}

// Record records a time measurement
func (m *Measurement) Record(name string, duration time.Duration) {
	r := m.Result(name)
	r.WithSpeed = false
	r.Durations = append(r.Durations, duration)
}

// RecordSpeed records a time measurement that can be expressed in speed
func (m *Measurement) RecordSpeed(name string, duration time.Duration) {
	r := m.Result(name)
	r.WithSpeed = true
	r.Durations = append(r.Durations, duration)
}

// PrintStats prints important valueas about the measurement
func (m *Measurement) PrintStats(w io.Writer) {
	const binCount = 10

	type Hist struct {
		*Result
		*hrtime.Histogram
	}

	hists := []Hist{}
	for _, result := range m.Results {
		hists = append(hists, Hist{
			Result:    result,
			Histogram: hrtime.NewDurationHistogram(result.Durations, binCount),
		})
	}

	sec := func(ns float64) string {
		return fmt.Sprintf("%.2f", ns/1e9)
	}
	speed := func(ns float64) string {
		return fmt.Sprintf("%.2f", m.Size.MB()/(ns/1e9))
	}

	for _, hist := range hists {
		if !hist.WithSpeed {
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				m.Size, hist.Name,
				sec(hist.Average), "",
				sec(hist.Maximum), "",
				sec(hist.P50), "",
				sec(hist.P90), "",
				sec(hist.P99), "",
			)
			continue
		}
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			m.Size, hist.Name,
			sec(hist.Average), speed(hist.Average),
			sec(hist.Maximum), speed(hist.Maximum),
			sec(hist.P50), speed(hist.P50),
			sec(hist.P90), speed(hist.P90),
			sec(hist.P99), speed(hist.P99),
		)
	}
}

// Benchmark runs benchmarks on bucket with given size
func Benchmark(client Client, bucket string, size memory.Size, count int, duration time.Duration) (Measurement, error) {
	log.Print("Benchmarking size ", size.String(), " ")

	data := make([]byte, size.Int())
	result := make([]byte, size.Int())

	defer fmt.Println()

	measurement := Measurement{}
	measurement.Size = size
	start := time.Now()
	for k := 0; k < count; k++ {
		if time.Since(start) > duration {
			break
		}
		fmt.Print(".")

		// rand.Read(data[:])
		for i := range data {
			data[i] = 'a' + byte(i%26)
		}

		{ // uploading
			start := hrtime.Now()
			err := client.Upload(bucket, "data", data)
			finish := hrtime.Now()
			if err != nil {
				return measurement, fmt.Errorf("upload failed: %+v", err)
			}

			measurement.RecordSpeed("Upload", finish-start)
		}

		{ // downloading
			start := hrtime.Now()
			var err error
			result, err = client.Download(bucket, "data", result)
			if err != nil {
				return measurement, fmt.Errorf("get object failed: %+v", err)
			}
			finish := hrtime.Now()

			if !bytes.Equal(data, result) {
				return measurement, fmt.Errorf("upload/download do not match: lengths %d and %d", len(data), len(result))
			}

			measurement.RecordSpeed("Download", finish-start)
		}

		{ // deleting
			start := hrtime.Now()
			err := client.Delete(bucket, "data")
			if err != nil {
				return measurement, fmt.Errorf("delete failed: %+v", err)
			}
			finish := hrtime.Now()

			measurement.Record("Delete", finish-start)
		}
	}

	return measurement, nil
}

// Config is the setup for a particular client
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	NoSSL     bool
}

// Client is the common interface for different implementations
type Client interface {
	MakeBucket(bucket, location string) error
	RemoveBucket(bucket string) error
	ListBuckets() ([]string, error)

	Upload(bucket, objectName string, data []byte) error
	UploadMultipart(bucket, objectName string, data []byte, multipartThreshold int) error
	Download(bucket, objectName string, buffer []byte) ([]byte, error)
	Delete(bucket, objectName string) error
	ListObjects(bucket, prefix string) ([]string, error)
}
