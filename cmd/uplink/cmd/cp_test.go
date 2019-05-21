// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd_test

import (
	"crypto/rand"
	"log"
	"os"
	"strconv"
	"testing"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/s3client"
)

const (
	bucket                = "testbucket"
	uplinkEncryptionKey   = "supersecretkey"
	storjSimSatelliteAddr = "127.0.0.1:10000"
)

func setup() s3client.Client {
	var conf s3client.Config
	conf.Satellite = storjSimSatelliteAddr
	conf.EncryptionKey = uplinkEncryptionKey
	conf.APIKey = os.Getenv("storjSimApiKey")

	client, err := s3client.NewUplink(conf)
	if err != nil {
		log.Fatalf("failed to create NewUplink: %+v\n", err)
	}
	err = client.MakeBucket(bucket, "")
	if err != nil {
		log.Fatalf("failed to create bucket %q: %+v\n", bucket, err)
	}
	return client
}

func teardown(client s3client.Client) {
	for _, bm := range benchmarkCases {
		deleteUploadedFiles(bm.name)
	}

	err := client.RemoveBucket(bucket)
	if err != nil {
		log.Fatalf("failed to remove bucket %q: %+v\n", bucket, err)
	}
}

func deleteUploadedFiles(name string) {
	for filePath := range uploadedFiles[name] {
		err := client.Delete(bucket, filePath)
		if err != nil {
			log.Printf("failed to delete file %q: %+v\n", filePath, err)
		}
	}
}

var client = setup()

var benchmarkCases = []struct {
	name     string
	filesize memory.Size
}{
	{"100B", 100 * memory.B},
	{"1MB", 1 * memory.MiB},
	{"10MB", 10 * memory.MiB},
	{"100MB", 100 * memory.MiB},
	{"1G", 1 * memory.GiB},
}

var uploadedFiles = map[string]map[string]struct{}{}

func BenchmarkUpload(b *testing.B) {
	for _, bm := range benchmarkCases {
		b.Run(bm.name, func(b *testing.B) {
			file := createRandomBytes(bm.filesize)
			b.SetBytes(1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filePath := "folder/data" + strconv.Itoa(i) + "_" + bm.name
				err := client.Upload(bucket, filePath, file)
				if err != nil {
					log.Fatalf("failed to upload file %q: %+v\n", filePath, err)
				}
				if uploadedFiles[bm.name] == nil {
					uploadedFiles[bm.name] = make(map[string]struct{})
				}
				uploadedFiles[bm.name][filePath] = struct{}{}
			}
		})
	}
}

func BenchmarkDownload(b *testing.B) {
	for _, bm := range benchmarkCases {
		b.Run(bm.name, func(b *testing.B) {
			var file string
			for filePath := range uploadedFiles[bm.name] {
				file = filePath
			}
			buf := make([]byte, bm.filesize)
			b.SetBytes(1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := client.Download(bucket, file, buf)
				if err != nil {
					log.Fatalf("failed to download file %q: %+v\n", file, err)
				}
			}
		})
	}

	teardown(client)
}

func createRandomBytes(filesize memory.Size) []byte {
	buf := make([]byte, filesize)
	rand.Read(buf)
	return buf
}
