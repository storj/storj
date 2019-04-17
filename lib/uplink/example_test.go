// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
)

const (
	testAPIKey        = "test-key"
	testProject       = "test-project"
	testBucket        = "test-bucket"
	testUploadPath    = "foo/bar/baz"
	testDataSize      = 8 * memory.KiB
	testEncryptionKey = "you'll never guess this"
)

func Example() {
	ctx := context.Background()

	// Set up test planet
	planet, err := testplanet.NewWithLogger(zap.L(), 1, 5, 1)
	if err != nil {
		log.Fatalln("could not create new test planet:", err)
	}

	// Start test planet
	planet.Start(ctx)

	// Create a project on the satellite
	project, err := planet.Satellites[0].DB.Console().Projects().Insert(ctx, &console.Project{
		Name: testProject,
	})
	if err != nil {
		log.Fatalln("could not create project:", err)
	}

	// Create an API key for the project
	apiKey := console.APIKey{}
	apiKeyInfo := console.APIKeyInfo{
		ProjectID: project.ID,
		Name:      testAPIKey,
	}
	_, err = planet.Satellites[0].DB.Console().APIKeys().Create(ctx, apiKey, apiKeyInfo)
	if err != nil {
		log.Fatalln("could not create api key:", err)
	}

	parsedAPIKey, err := uplink.ParseAPIKey(apiKey.String())
	if err != nil {
		log.Fatalln("could not parse api key:", err)
	}

	encryptionKey := new(storj.Key)
	copy(encryptionKey[:], []byte(testEncryptionKey))

	// Skipping CA whitelist is necessary for test planet, but not for real use of libuplink
	cfg := uplink.Config{}
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true

	// Create an Uplink object with a default config
	upl, err := uplink.NewUplink(ctx, &cfg)
	if err != nil {
		log.Fatalln("could not create new Uplink object:", err)
	}
	defer upl.Close()

	// It is temporarily required to set the encryption key in project options.
	// This requirement will be removed in the future.
	opts := uplink.ProjectOptions{}
	opts.Volatile.EncryptionKey = encryptionKey

	// Open up the Project we will be working with
	proj, err := upl.OpenProject(ctx, planet.Satellites[0].Addr(), parsedAPIKey, &opts)
	if err != nil {
		log.Fatalf("could not open project: %+v\n", err)
	}
	defer proj.Close()

	// Set the bucket's redundancy scheme to the one of the test planet's uplink.
	// This is only necessary for test planet, but not for real use of libuplink.
	redundancyCfg := planet.Uplinks[0].GetConfig(planet.Satellites[0]).RS
	bucketCfg := uplink.BucketConfig{}
	bucketCfg.Volatile.RedundancyScheme = storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		ShareSize:      redundancyCfg.ErasureShareSize.Int32(),
		RequiredShares: int16(redundancyCfg.MinThreshold),
		RepairShares:   int16(redundancyCfg.RepairThreshold),
		OptimalShares:  int16(redundancyCfg.SuccessThreshold),
		TotalShares:    int16(redundancyCfg.MaxThreshold),
	}

	// Create the desired Bucket within the Project
	_, err = proj.CreateBucket(ctx, testBucket, &bucketCfg)
	if err != nil {
		log.Fatalln("could not create bucket:", err)
	}

	// Open up the desired Bucket within the Project
	bucket, err := proj.OpenBucket(ctx, testBucket, &uplink.EncryptionAccess{Key: *encryptionKey})
	if err != nil {
		log.Fatalf("could not open bucket %q: %v\n", testBucket, err)
	}
	defer bucket.Close()

	testData, err := ioutil.ReadAll(io.LimitReader(rand.Reader, testDataSize.Int64()))
	if err != nil {
		log.Fatalln("could not generate test data", err)
	}

	// Upload our Object to the specified path
	buf := bytes.NewBuffer(testData)
	err = bucket.UploadObject(ctx, testUploadPath, buf, nil)
	if err != nil {
		log.Fatalln("could not upload:", err)
	}

	// Initiate a download of the same object again
	readBack, err := bucket.OpenObject(ctx, testUploadPath)
	if err != nil {
		log.Fatalf("could not open object at %q: %v\n", testUploadPath, err)
	}
	defer readBack.Close()

	// We want the whole thing, so range from 0 to -1
	strm, err := readBack.DownloadRange(ctx, 0, -1)
	if err != nil {
		log.Fatalln("could not initiate download:", err)
	}
	defer strm.Close()

	// Read everything from the stream
	receivedContents, err := ioutil.ReadAll(strm)
	if err != nil {
		log.Fatalln("could not read object:", err)
	}

	if !bytes.Equal(receivedContents, testData) {
		log.Fatalf("got different object back: %q != %q\n", []byte(testData), receivedContents)
	}

	fmt.Println("success!")

	// Output:
	// success!
}
