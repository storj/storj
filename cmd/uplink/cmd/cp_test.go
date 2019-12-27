// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd_test

import (
	"encoding/hex"
	"errors"
	"log"
	"os"
	"sync"
	"testing"

	"storj.io/common/memory"
	"storj.io/common/testrand"
	"storj.io/storj/private/s3client"
)

var benchmarkCases = []struct {
	name       string
	objectsize memory.Size
}{
	{"100B", 100 * memory.B},
	{"1MB", 1 * memory.MiB},
	{"10MB", 10 * memory.MiB},
	{"100MB", 100 * memory.MiB},
	{"1G", 1 * memory.GiB},
}

var testobjectData struct {
	sync.Once
	objects map[string][]byte
}

// testObjects returns test objects (i.e. slice of bytes) that
// will be used as the objects to upload/download tests
func testObjects() map[string][]byte {
	testobjectData.Do(func() {
		objects := make(map[string][]byte)
		for _, bm := range benchmarkCases {
			objects[bm.name] = testrand.Bytes(bm.objectsize)
		}
		testobjectData.objects = objects
	})
	return testobjectData.objects
}

// uplinkSetup setups an uplink to use for testing uploads/downloads
func uplinkSetup(bucket string) s3client.Client {
	conf, err := setupConfig()
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

func setupConfig() (s3client.Config, error) {
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

func getEnvOrDefault(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func BenchmarkUpload_Uplink(b *testing.B) {
	bucket := "testbucket"
	client := uplinkSetup(bucket)

	// uploadedObjects is used to store the names of all objects that are uploaded
	// so that we can make sure to delete them all during cleanup
	var uploadedObjects = map[string][]string{}

	uploadedObjects = benchmarkUpload(b, client, bucket, uploadedObjects)

	teardown(client, bucket, uploadedObjects)
}

func teardown(client s3client.Client, bucket string, uploadedObjects map[string][]string) {
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

func BenchmarkDownload_Uplink(b *testing.B) {
	bucket := "testbucket"
	client := uplinkSetup(bucket)

	// upload some test objects so that there is something to download
	uploadTestObjects(client, bucket)

	benchmarkDownload(b, bucket, client)

	teardownTestObjects(client, bucket)
}

func uploadTestObjects(client s3client.Client, bucket string) {
	for name, data := range testObjects() {
		objectName := "folder/data_" + name
		err := client.Upload(bucket, objectName, data)
		if err != nil {
			log.Fatalf("failed to upload object %q: %+v\n", objectName, err)
		}
	}
}

func teardownTestObjects(client s3client.Client, bucket string) {
	for name := range testObjects() {
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

func benchmarkUpload(b *testing.B, client s3client.Client, bucket string, uploadedObjects map[string][]string) map[string][]string {
	for _, bm := range benchmarkCases {
		benchmark := bm
		b.Run(benchmark.name, func(b *testing.B) {
			b.SetBytes(benchmark.objectsize.Int64())
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// make some random bytes so the objectPath is unique
				randomBytes := testrand.Bytes(16)
				uniquePathPart := hex.EncodeToString(randomBytes)
				objectPath := "folder/data" + uniquePathPart + "_" + benchmark.name
				err := client.Upload(bucket, objectPath, testObjects()[benchmark.name])
				if err != nil {
					log.Fatalf("failed to upload object %q: %+v\n", objectPath, err)
				}
				if uploadedObjects[benchmark.name] == nil {
					uploadedObjects[benchmark.name] = []string{}
				}
				uploadedObjects[benchmark.name] = append(uploadedObjects[benchmark.name], objectPath)
			}
		})
	}
	return uploadedObjects
}

func benchmarkDownload(b *testing.B, bucket string, client s3client.Client) {
	for _, bm := range benchmarkCases {
		benchmark := bm
		b.Run(benchmark.name, func(b *testing.B) {
			buf := make([]byte, benchmark.objectsize)
			b.SetBytes(benchmark.objectsize.Int64())
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				objectName := "folder/data_" + benchmark.name
				out, err := client.Download(bucket, objectName, buf)
				if err != nil {
					log.Fatalf("failed to download object %q: %+v\n", objectName, err)
				}
				expectedBytes := benchmark.objectsize.Int()
				actualBytes := len(out)
				if actualBytes != expectedBytes {
					log.Fatalf("err downloading object %q: Expected %d bytes, but got actual bytes: %d\n",
						objectName, expectedBytes, actualBytes,
					)
				}
			}
		})
	}
}

func s3ClientSetup(bucket string) s3client.Client {
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
	const s3gateway = "https://s3.amazonaws.com/"
	var conf s3client.Config
	conf.S3Gateway = s3gateway
	conf.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	conf.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	if conf.AccessKey == "" || conf.SecretKey == "" {
		return conf, errors.New("expecting environment variables $AWS_ACCESS_KEY_ID and $AWS_SECRET_ACCESS_KEY to be set")
	}
	return conf, nil
}

func BenchmarkUpload_S3(b *testing.B) {
	bucket := "testbucket3bgdp2xbkkflxc2tallstvh6pb824r"
	var client = s3ClientSetup(bucket)

	// uploadedObjects is used to store the names of all objects that are uploaded
	// so that we can make sure to delete them all during cleanup
	var uploadedObjects = map[string][]string{}

	uploadedObjects = benchmarkUpload(b, client, bucket, uploadedObjects)

	teardown(client, bucket, uploadedObjects)
}

func BenchmarkDownload_S3(b *testing.B) {
	bucket := "testbucket3bgdp2xbkkflxc2tallstvh6pb826a"
	var client = s3ClientSetup(bucket)

	// upload some test objects so that there is something to download
	uploadTestObjects(client, bucket)

	benchmarkDownload(b, bucket, client)

	teardownTestObjects(client, bucket)
}
