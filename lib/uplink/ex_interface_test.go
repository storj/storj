// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

func logClose(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}

// WorkWithLibUplink uploads the specified data to the specified path in the
// specified bucket, using the specified Satellite, encryption key, and API key.
func WorkWithLibUplink(satelliteAddress string, encryptionKey *storj.Key, apiKey uplink.APIKey,
	bucketName, uploadPath string, dataToUpload []byte) error {
	ctx := context.Background()

	// Create an Uplink object with a default config
	upl, err := uplink.NewUplink(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not create new Uplink object: %v", err)
	}
	defer logClose(upl.Close)

	// Open up the Project we will be working with
	proj, err := upl.OpenProject(ctx, satelliteAddress, apiKey)
	if err != nil {
		return fmt.Errorf("could not open project: %v", err)
	}
	defer logClose(proj.Close)

	// Create the desired Bucket within the Project
	_, err = proj.CreateBucket(ctx, bucketName, nil)
	if err != nil {
		return fmt.Errorf("could not create bucket: %v", err)
	}

	// Open up the desired Bucket within the Project
	bucket, err := proj.OpenBucket(ctx, bucketName, uplink.NewEncryptionAccessWithDefaultKey(*encryptionKey))
	if err != nil {
		return fmt.Errorf("could not open bucket %q: %v", bucketName, err)
	}
	defer logClose(bucket.Close)

	// Upload our Object to the specified path
	buf := bytes.NewBuffer(dataToUpload)
	err = bucket.UploadObject(ctx, uploadPath, buf, nil)
	if err != nil {
		return fmt.Errorf("could not upload: %v", err)
	}

	// Initiate a download of the same object again
	readBack, err := bucket.OpenObject(ctx, uploadPath)
	if err != nil {
		return fmt.Errorf("could not open object at %q: %v", uploadPath, err)
	}
	defer logClose(readBack.Close)

	// We want the whole thing, so range from 0 to -1
	strm, err := readBack.DownloadRange(ctx, 0, -1)
	if err != nil {
		return fmt.Errorf("could not initiate download: %v", err)
	}
	defer logClose(strm.Close)

	// Read everything from the stream
	receivedContents, err := ioutil.ReadAll(strm)
	if err != nil {
		return fmt.Errorf("could not read object: %v", err)
	}

	if !bytes.Equal(receivedContents, dataToUpload) {
		return fmt.Errorf("got different object back: %q != %q", dataToUpload, receivedContents)
	}
	return nil
}

func Example_interface() {

	const (
		myAPIKey = "change-me-to-the-api-key-created-in-satellite-gui"

		satellite       = "mars.tardigrade.io:7777"
		myBucket        = "my-first-bucket"
		myUploadPath    = "foo/bar/baz"
		myData          = "one fish two fish red fish blue fish"
		myEncryptionKey = "you'll never guess this"
	)

	var encryptionKey storj.Key
	copy(encryptionKey[:], []byte(myEncryptionKey))

	apiKey, err := uplink.ParseAPIKey(myAPIKey)
	if err != nil {
		log.Fatal("could not parse api key:", err)
	}

	err = WorkWithLibUplink(satellite, &encryptionKey, apiKey, myBucket, myUploadPath, []byte(myData))
	if err != nil {
		log.Fatal("error:", err)
	}

	fmt.Println("success!")
}
