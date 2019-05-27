// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd_test

import (
	"encoding/hex"
	"errors"
	"log"
	"math/rand"
	"os"
	"testing"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/s3client"
)

const (
	bucket = "testbucket3bgdp2xbkkflxc2tallstvh6pb824r7zc"
)

var benchmarkCases = []struct {
	name       string
	objectsize memory.Size
}{
	{"100B", 100 * memory.B},
	// {"1MB", 1 * memory.MiB},
	// {"10MB", 10 * memory.MiB},
	// {"100MB", 100 * memory.MiB},
	// {"1G", 1 * memory.GiB},
}

// uplinkSetup setups an uplink to use for testing uploads/downloads
func uplinkSetup() s3client.Client {
	conf, err := uplinkConfigSetup()
	if err != nil {
		log.Fatalf("failed to setup s3client config: %+v\n", err)
	}
	client, err := s3client.NewUplink(conf)
	if err != nil {
		log.Fatalf("failed to create s3client NewUplink: %+v\n", err)
	}
	err = client.MakeBucket(bucket, "")
	if err != nil {
		log.Fatalf("failed to create bucket with s3client %q: %+v\n", bucket, err)
	}
	return client
}

func uplinkConfigSetup() (s3client.Config, error) {
	const (
		uplinkEncryptionKey  = "supersecretkey"
		defaultSatelliteAddr = "127.0.0.1:10000"
	)
	var conf s3client.Config
	conf.EncryptionKey = uplinkEncryptionKey
	conf.Satellite = getEnvOrDefault("SATELLITE_0_ADDR", defaultSatelliteAddr)
	conf.APIKey = getEnvOrDefault("GATEWAY_0_API_KEY", os.Getenv("apiKey"))
	if conf.APIKey == "" {
		return conf, errors.New("no api key provided. Expecting an env var $GATEWAY_0_API_KEY or $apiKey")
	}
	return conf, nil
}

func s3ClientSetup() s3client.Client {
	conf, err := s3ClientConfigSetup()
	if err != nil {
		log.Fatalf("failed to setup s3client config: %+v\n", err)
	}
	client, err := s3client.NewAWSCLI(conf)
	if err != nil {
		log.Fatalf("failed to create s3client NewUplink: %+v\n", err)
	}
	const region = "us-east-1"
	err = client.MakeBucket(bucket, region)
	if err != nil {
		log.Fatalf("failed to create bucket with s3client %q: %+v\n", bucket, err)
	}
	return client
}

func s3ClientConfigSetup() (s3client.Config, error) {
	const (
		s3gateway = "https://s3.amazonaws.com/"
	)
	var conf s3client.Config
	conf.S3Gateway = s3gateway
	conf.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	conf.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	if conf.AccessKey == "" || conf.SecretKey == "" {
		return conf, errors.New("expecting environment variables $AWS_ACCESS_KEY_ID and $AWS_SECRET_ACCESS_KEY to be set")
	}
	return conf, nil
}

func getEnvOrDefault(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func teardown(client s3client.Client, uploadedObjects map[string][]string) {
	for _, bm := range benchmarkCases {
		for _, objectPath := range uploadedObjects[bm.name] {
			err := client.Delete(bucket, objectPath)
			if err != nil {
				log.Printf("failed to delete object %q: %+v\n", objectPath, err)
			}
		}
	}

	err := client.RemoveBucket(bucket)
	if err != nil {
		log.Fatalf("failed to remove bucket %q: %+v\n", bucket, err)
	}
}

var testObjects = createObjects()

// createObjects generates the objects (i.e. slice of bytes) that
// will be used as the objects to upload/download tests
func createObjects() map[string][]byte {
	objects := make(map[string][]byte)
	for _, bm := range benchmarkCases {
		data := make([]byte, bm.objectsize)
		_, err := rand.Read(data)
		if err != nil {
			log.Fatalf("failed to read random bytes: %+v\n", err)
		}
		objects[bm.name] = data
	}
	return objects
}

func uploadTestObjects(client s3client.Client) {
	for name, data := range testObjects {
		objectName := "folder/data_" + name
		err := client.Upload(bucket, objectName, data)
		if err != nil {
			log.Fatalf("failed to upload object %q: %+v\n", objectName, err)
		}
	}
}

func teardownTestObjects(client s3client.Client) {
	for name := range testObjects {
		objectName := "folder/data_" + name
		err := client.Delete(bucket, objectName)
		if err != nil {
			log.Fatalf("failed to delete object %q: %+v\n", objectName, err)
		}
	}
	err := client.RemoveBucket(bucket)
	if err != nil {
		log.Fatalf("failed to remove bucket %q: %+v\n", bucket, err)
	}
}

func benchmarkUpload(b *testing.B, client s3client.Client, uploadedObjects map[string][]string) map[string][]string {
	for _, bm := range benchmarkCases {
		b.Run(bm.name, func(b *testing.B) {
			b.SetBytes(bm.objectsize.Int64())
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// make some random bytes so the objectPath is unique
				randomBytes := make([]byte, 16)
				rand.Read(randomBytes)
				uniquePathPart := hex.EncodeToString(randomBytes)
				objectPath := "folder/data" + uniquePathPart + "_" + bm.name
				err := client.Upload(bucket, objectPath, testObjects[bm.name])
				if err != nil {
					log.Fatalf("failed to upload object %q: %+v\n", objectPath, err)
				}
				if uploadedObjects[bm.name] == nil {
					uploadedObjects[bm.name] = []string{}
				}
				uploadedObjects[bm.name] = append(uploadedObjects[bm.name], objectPath)
			}
		})
	}
	return uploadedObjects
}

func benchmarkDownload(b *testing.B, client s3client.Client) {
	for _, bm := range benchmarkCases {
		b.Run(bm.name, func(b *testing.B) {
			buf := make([]byte, bm.objectsize)
			b.SetBytes(bm.objectsize.Int64())
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				objectName := "folder/data_" + bm.name
				out, err := client.Download(bucket, objectName, buf)
				if err != nil {
					log.Fatalf("failed to download object %q: %+v\n", objectName, err)
				}
				expectedBytes := bm.objectsize.Int()
				actualBytes := len(out)
				if actualBytes != expectedBytes {
					log.Fatalf("err downloading object %q: Excepted %d bytes, but got actual bytes: %d\n",
						objectName, expectedBytes, actualBytes,
					)
				}
			}
		})
	}
}

func BenchmarkUpload_Uplink(b *testing.B) {
	client := uplinkSetup()

	// uploadedObjects is used to store the names of all objects that are uploaded
	// so that we can make sure to delete them all during cleanup
	var uploadedObjects = map[string][]string{}

	uploadedObjects = benchmarkUpload(b, client, uploadedObjects)

	teardown(client, uploadedObjects)
}

func BenchmarkDownload_Uplink(b *testing.B) {
	var client = uplinkSetup()

	// upload some test objects so that there is something to download
	uploadTestObjects(client)

	benchmarkDownload(b, client)

	teardownTestObjects(client)
}

func BenchmarkUpload_S3(b *testing.B) {
	var client = s3ClientSetup()

	// uploadedObjects is used to store the names of all objects that are uploaded
	// so that we can make sure to delete them all during cleanup
	var uploadedObjects = map[string][]string{}

	uploadedObjects = benchmarkUpload(b, client, uploadedObjects)

	teardown(client, uploadedObjects)
}

func BenchmarkDownload_S3(b *testing.B) {
	var client = s3ClientSetup()

	// upload some test objects so that there is something to download
	uploadTestObjects(client)

	benchmarkDownload(b, client)

	teardownTestObjects(client)
}
