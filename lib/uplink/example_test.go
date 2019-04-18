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

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
)

const (
	testProject       = "test-project"
	testBucket        = "test-bucket"
	testUploadPath    = "foo/bar/baz"
	testDataSize      = 8 * memory.KiB
	testEncryptionKey = "you'll never guess this"
)

func Example() {
	setupTestEnv(func(ctx context.Context, satelliteAddr, apiKey string, redundancy storj.RedundancyScheme) (err error) {
		parsedAPIKey, err := uplink.ParseAPIKey(apiKey)
		if err != nil {
			return fmt.Errorf("count not parse api key: %+v", err)
		}

		encryptionKey := new(storj.Key)
		copy(encryptionKey[:], []byte(testEncryptionKey))

		// Skipping CA whitelist is necessary for test planet, but not for real use of libuplink
		cfg := uplink.Config{}
		cfg.Volatile.TLS.SkipPeerCAWhitelist = true

		// Create an Uplink object with a default config
		upl, err := uplink.NewUplink(ctx, &cfg)
		if err != nil {
			return fmt.Errorf("could not create new Uplink object: %+v", err)
		}
		defer func() { err = errs.Combine(err, upl.Close()) }()

		// It is temporarily required to set the encryption key in project options.
		// This requirement will be removed in the future.
		opts := uplink.ProjectOptions{}
		opts.Volatile.EncryptionKey = encryptionKey

		// Open up the Project we will be working with
		proj, err := upl.OpenProject(ctx, satelliteAddr, parsedAPIKey, &opts)
		if err != nil {
			return fmt.Errorf("could not open project: %+v", err)
		}
		defer func() { err = errs.Combine(err, proj.Close()) }()

		// Set the bucket's redundancy scheme to the one of the test planet's uplink.
		// This is only necessary for test planet, but not for real use of libuplink.
		bucketCfg := uplink.BucketConfig{}
		bucketCfg.Volatile.RedundancyScheme = redundancy

		// Create the desired Bucket within the Project
		_, err = proj.CreateBucket(ctx, testBucket, &bucketCfg)
		if err != nil {
			return fmt.Errorf("could not create bucket: %+v", err)
		}

		// Open up the desired Bucket within the Project
		bucket, err := proj.OpenBucket(ctx, testBucket, &uplink.EncryptionAccess{Key: *encryptionKey})
		if err != nil {
			return fmt.Errorf("could not open bucket: %+v", err)
		}
		defer func() { err = errs.Combine(err, bucket.Close()) }()

		testData, err := ioutil.ReadAll(io.LimitReader(rand.Reader, testDataSize.Int64()))
		if err != nil {
			return fmt.Errorf("could not generate test data: %+v", err)
		}

		// Upload our Object to the specified path
		buf := bytes.NewBuffer(testData)
		err = bucket.UploadObject(ctx, testUploadPath, buf, nil)
		if err != nil {
			return fmt.Errorf("could not upload: %+v", err)
		}

		// Initiate a download of the same object again
		readBack, err := bucket.OpenObject(ctx, testUploadPath)
		if err != nil {
			return fmt.Errorf("could not open object: %+v", err)
		}
		defer func() { err = errs.Combine(err, readBack.Close()) }()

		// We want the whole thing, so range from 0 to -1
		strm, err := readBack.DownloadRange(ctx, 0, -1)
		if err != nil {
			return fmt.Errorf("could not initiate download: %+v", err)
		}
		defer func() { err = errs.Combine(err, strm.Close()) }()

		// Read everything from the stream
		receivedContents, err := ioutil.ReadAll(strm)
		if err != nil {
			return fmt.Errorf("could not read object: %+v", err)
		}

		if !bytes.Equal(receivedContents, testData) {
			return fmt.Errorf("got different object back: %q != %q", []byte(testData), receivedContents)
		}

		// Delete the Object
		err = bucket.DeleteObject(ctx, testUploadPath)
		if err != nil {
			return fmt.Errorf("could not delete object: %+v", err)
		}

		// Delete the Bucket
		err = proj.DeleteBucket(ctx, testBucket)
		if err != nil {
			return fmt.Errorf("could not delete bucket: %+v", err)
		}

		return nil
	})

	fmt.Println("success!")

	// Output:
	// success!
}

func setupTestEnv(test func(context.Context, string, string, storj.RedundancyScheme) error) {
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
		Name:      "Test Key",
	}
	_, err = planet.Satellites[0].DB.Console().APIKeys().Create(ctx, apiKey, apiKeyInfo)
	if err != nil {
		log.Fatalln("could not create api key:", err)
	}

	redundancyCfg := planet.Uplinks[0].GetConfig(planet.Satellites[0]).RS
	redundancy := storj.RedundancyScheme{
		Algorithm:      storj.ReedSolomon,
		ShareSize:      redundancyCfg.ErasureShareSize.Int32(),
		RequiredShares: int16(redundancyCfg.MinThreshold),
		RepairShares:   int16(redundancyCfg.RepairThreshold),
		OptimalShares:  int16(redundancyCfg.SuccessThreshold),
		TotalShares:    int16(redundancyCfg.MaxThreshold),
	}

	err = test(ctx, planet.Satellites[0].Addr(), apiKey.String(), redundancy)
	if err != nil {
		log.Fatalln("test error:", err)
	}
}
