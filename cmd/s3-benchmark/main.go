// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/loov/hrtime"

	"storj.io/common/memory"
	"storj.io/storj/private/s3client"
)

func main() {
	var conf s3client.Config

	flag.StringVar(&conf.S3Gateway, "s3-gateway", "127.0.0.1:7777", "s3 gateway address")
	flag.StringVar(&conf.Satellite, "satellite", "127.0.0.1:7778", "satellite address")
	flag.StringVar(&conf.AccessKey, "accesskey", "insecure-dev-access-key", "access key")
	flag.StringVar(&conf.SecretKey, "secretkey", "insecure-dev-secret-key", "secret key")
	flag.StringVar(&conf.APIKey, "apikey", "abc123", "api key")
	flag.StringVar(&conf.EncryptionKey, "encryptionkey", "abc123", "encryption key")
	flag.BoolVar(&conf.NoSSL, "no-ssl", false, "disable ssl")
	flag.StringVar(&conf.ConfigDir, "config-dir", "", "path of config dir to use. If empty, a config will be created.")

	clientName := flag.String("client", "minio", "client to use for requests (supported: minio, aws-cli, uplink)")

	location := flag.String("location", "", "bucket location")
	count := flag.Int("count", 50, "benchmark count")
	duration := flag.Duration("time", 2*time.Minute, "maximum benchmark time per filesize")

	suffix := time.Now().Format("-2006-01-02-150405")

	plotname := flag.String("plot", "plot"+suffix+".svg", "plot results")

	filesizes := &memory.Sizes{
		Default: []memory.Size{
			1 * memory.KiB,
			256 * memory.KiB,
			1 * memory.MiB,
			32 * memory.MiB,
			64 * memory.MiB,
			256 * memory.MiB,
		},
	}
	flag.Var(filesizes, "filesize", "filesizes to test with")
	listsize := flag.Int("listsize", 1000, "listsize to test with")

	flag.Parse()

	var client s3client.Client
	var err error

	switch *clientName {
	default:
		log.Println("unknown client name ", *clientName, " defaulting to minio")
		fallthrough
	case "minio":
		client, err = s3client.NewMinio(conf)
	case "aws-cli":
		client, err = s3client.NewAWSCLI(conf)
	case "uplink":
		client, err = s3client.NewUplink(conf)
	}
	if err != nil {
		log.Fatal(err)
	}

	bucket := "benchmark" + suffix
	log.Println("Creating bucket", bucket)

	// 1 bucket for file up and downloads
	err = client.MakeBucket(bucket, *location)
	if err != nil {
		log.Fatalf("failed to create bucket %q: %+v\n", bucket, err)
	}

	data := make([]byte, 1)
	log.Println("Creating files", bucket)
	// n files in one folder
	for k := 0; k < *listsize; k++ {
		err := client.Upload(bucket, "folder/data"+strconv.Itoa(k), data)
		if err != nil {
			log.Fatalf("failed to create file %q: %+v\n", "folder/data"+strconv.Itoa(k), err)
		}
	}

	log.Println("Creating folders", bucket)
	// n - 1 (one folder already exists) folders with one file in each folder
	for k := 0; k < *listsize-1; k++ {
		err := client.Upload(bucket, "folder"+strconv.Itoa(k)+"/data", data)
		if err != nil {
			log.Fatalf("failed to create folder %q: %+v\n", "folder"+strconv.Itoa(k)+"/data", err)
		}
	}

	defer func() {
		log.Println("Removing files")
		for k := 0; k < *listsize; k++ {
			err := client.Delete(bucket, "folder/data"+strconv.Itoa(k))
			if err != nil {
				log.Fatalf("failed to delete file %q: %+v\n", "folder/data"+strconv.Itoa(k), err)
			}
		}

		log.Println("Removing folders")
		for k := 0; k < *listsize-1; k++ {
			err := client.Delete(bucket, "folder"+strconv.Itoa(k)+"/data")
			if err != nil {
				log.Fatalf("failed to delete folder %q: %+v\n", "folder"+strconv.Itoa(k)+"/data", err)
			}
		}

		log.Println("Removing bucket")
		err := client.RemoveBucket(bucket)
		if err != nil {
			log.Fatalf("failed to remove bucket %q", bucket)
		}

	}()

	measurements := []Measurement{}
	measurement, err := ListBenchmark(client, bucket, *listsize, *count, *duration)
	if err != nil {
		log.Fatal(err)
	}
	measurements = append(measurements, measurement)
	for _, filesize := range filesizes.Sizes() {
		measurement, err := FileBenchmark(client, bucket, filesize, *count, *duration)
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

// FileBenchmark runs file upload, download and delete benchmarks on bucket with given filesize
func FileBenchmark(client s3client.Client, bucket string, filesize memory.Size, count int, duration time.Duration) (Measurement, error) {
	log.Print("Benchmarking file size ", filesize.String(), " ")

	data := make([]byte, filesize.Int())
	result := make([]byte, filesize.Int())

	defer fmt.Println()

	measurement := Measurement{}
	measurement.Size = filesize
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

// ListBenchmark runs list buckets, folders and files benchmarks on bucket
func ListBenchmark(client s3client.Client, bucket string, listsize int, count int, duration time.Duration) (Measurement, error) {
	log.Print("Benchmarking list")
	defer fmt.Println()
	measurement := Measurement{}
	//measurement.Size = listsize
	for k := 0; k < count; k++ {
		{ // list folders
			start := hrtime.Now()
			result, err := client.ListObjects(bucket, "")
			if err != nil {
				return measurement, fmt.Errorf("list folders failed: %+v", err)
			}
			finish := hrtime.Now()
			if len(result) != listsize {
				return measurement, fmt.Errorf("list folders result wrong: %+v", len(result))
			}
			measurement.Record("List Folders", finish-start)
		}
		{ // list files
			start := hrtime.Now()
			result, err := client.ListObjects(bucket, "folder")
			if err != nil {
				return measurement, fmt.Errorf("list files failed: %+v", err)
			}
			finish := hrtime.Now()
			if len(result) != listsize {
				return measurement, fmt.Errorf("list files result to low: %+v", len(result))
			}
			measurement.Record("List Files", finish-start)
		}
	}
	return measurement, nil
}
