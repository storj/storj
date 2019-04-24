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

const (
	myAPIKey = "change-me-to-the-api-key-created-in-satellite-gui"

	satellite       = "mars.tardigrade.io:7777"
	myBucket        = "my-first-bucket"
	myUploadPath    = "foo/bar/baz"
	myData          = "one fish two fish red fish blue fish"
	myEncryptionKey = "you'll never guess this"
)

func logClose(fn func() error) {
	err := fn()
	if err != nil {
		log.Fatal(err)
	}
}

// ExampleProj_CreateBucket example documentation
func ExampleProject_CreateBucket() {
	var encryptionKey storj.Key
	copy(encryptionKey[:], []byte(myEncryptionKey))

	apiKey, err := uplink.ParseAPIKey(myAPIKey)
	if err != nil {
		log.Fatal("could not parse api key:", err)
	}

	ctx := context.Background()

	// Create an Uplink object with a default config
	upl, err := uplink.NewUplink(ctx, nil)
	if err != nil {
		log.Fatal("could not create new Uplink object:", err)
	}

	defer logClose(upl.Close)

	// It is temporarily required to set the encryption key in project options.
	// This requirement will be removed in the future.
	opts := uplink.ProjectOptions{}
	opts.Volatile.EncryptionKey = &encryptionKey

	// Open up the Project we will be working with
	proj, err := upl.OpenProject(ctx, satellite, apiKey, &opts)
	if err != nil {
		log.Fatal("could not open project:", err)
	}
	defer logClose(proj.Close)

	// Create the desired Bucket within the Project
	_, err = proj.CreateBucket(ctx, myBucket, nil)
	if err != nil {
		log.Fatal("could not create bucket:", err)
	}

	fmt.Println("success")
}

// ExampleProj_OpenBucket example documentation
func ExampleProject_OpenBucket() {
	var encryptionKey storj.Key
	copy(encryptionKey[:], []byte(myEncryptionKey))

	apiKey, err := uplink.ParseAPIKey(myAPIKey)
	if err != nil {
		log.Fatalln("could not parse api key:", err)
	}

	ctx := context.Background()

	// Create an Uplink object with a default config
	upl, err := uplink.NewUplink(ctx, nil)
	if err != nil {
		log.Fatal("could not create new Uplink object:", err)
	}
	defer logClose(upl.Close)

	// It is temporarily required to set the encryption key in project options.
	// This requirement will be removed in the future.
	opts := uplink.ProjectOptions{}
	opts.Volatile.EncryptionKey = &encryptionKey

	// Open up the Project we will be working with
	proj, err := upl.OpenProject(ctx, satellite, apiKey, &opts)
	if err != nil {
		log.Fatal("could not open project:", err)
	}
	defer logClose(proj.Close)

	// Create the desired Bucket within the Project
	_, err = proj.CreateBucket(ctx, myBucket, nil)
	if err != nil {
		log.Fatal("could not create bucket:", err)
	}
	// Open up the desired Bucket within the Project
	bucket, err := proj.OpenBucket(ctx, myBucket, &uplink.EncryptionAccess{Key: encryptionKey})
	if err != nil {
		log.Fatal("could not open bucket ", myBucket, ":", err)
	}
	defer logClose(bucket.Close)
	fmt.Println("success")
}

// ExampleBucket_UploadObject example documentation
func ExampleBucket_UploadObject() {
	var encryptionKey storj.Key
	copy(encryptionKey[:], []byte(myEncryptionKey))

	apiKey, err := uplink.ParseAPIKey(myAPIKey)
	if err != nil {
		log.Fatalln("could not parse api key:", err)
	}

	ctx := context.Background()

	// Create an Uplink object with a default config
	upl, err := uplink.NewUplink(ctx, nil)
	if err != nil {
		log.Fatal("could not create new Uplink object:", err)
	}
	defer logClose(upl.Close)

	// It is temporarily required to set the encryption key in project options.
	// This requirement will be removed in the future.
	opts := uplink.ProjectOptions{}
	opts.Volatile.EncryptionKey = &encryptionKey

	// Open up the Project we will be working with
	proj, err := upl.OpenProject(ctx, satellite, apiKey, &opts)
	if err != nil {
		log.Fatal("could not open project:", err)
	}
	defer logClose(proj.Close)

	// Create the desired Bucket within the Project
	_, err = proj.CreateBucket(ctx, myBucket, nil)
	if err != nil {
		log.Fatal("could not create bucket:", err)
	}
	// Open up the desired Bucket within the Project
	bucket, err := proj.OpenBucket(ctx, myBucket, &uplink.EncryptionAccess{Key: encryptionKey})
	if err != nil {
		log.Fatal("could not open bucket ", myBucket, ":", err)
	}
	defer logClose(bucket.Close)

	// Upload our Object to the specified path
	buf := bytes.NewBuffer([]byte(myData))
	err = bucket.UploadObject(ctx, myUploadPath, buf, nil)
	if err != nil {
		log.Fatal("could not upload: ", err)
	}

	fmt.Println("success")
}

// ExampleReadBack_DownloadRange example documentation
func ExampleObject_DownloadRange() {
	var encryptionKey storj.Key
	copy(encryptionKey[:], []byte(myEncryptionKey))

	apiKey, err := uplink.ParseAPIKey(myAPIKey)
	if err != nil {
		log.Fatal("could not parse api key:", err)
	}

	ctx := context.Background()

	// Create an Uplink object with a default config
	upl, err := uplink.NewUplink(ctx, nil)
	if err != nil {
		log.Fatal("could not create new Uplink object:", err)
	}
	defer logClose(upl.Close)

	// It is temporarily required to set the encryption key in project options.
	// This requirement will be removed in the future.
	opts := uplink.ProjectOptions{}
	opts.Volatile.EncryptionKey = &encryptionKey

	// Open up the Project we will be working with
	proj, err := upl.OpenProject(ctx, satellite, apiKey, &opts)
	if err != nil {
		log.Fatal("could not open project:", err)
	}
	defer logClose(proj.Close)

	// Create the desired Bucket within the Project
	_, err = proj.CreateBucket(ctx, myBucket, nil)
	if err != nil {
		log.Fatal("could not create bucket:", err)
	}
	// Open up the desired Bucket within the Project
	bucket, err := proj.OpenBucket(ctx, myBucket, &uplink.EncryptionAccess{Key: encryptionKey})
	if err != nil {
		log.Fatal("could not open bucket ", myBucket, ":", err)
	}
	defer logClose(bucket.Close)

	// Upload our Object to the specified path
	buf := bytes.NewBuffer([]byte(myData))
	err = bucket.UploadObject(ctx, myUploadPath, buf, nil)
	if err != nil {
		log.Fatal("could not upload: ", err)
	}
	// Initiate a download of the same object again
	readBack, err := bucket.OpenObject(ctx, myUploadPath)
	if err != nil {
		log.Fatal("could not open object at ", myUploadPath, ":", err)
	}
	defer logClose(readBack.Close)

	// We want the whole thing, so range from 0 to -1
	strm, err := readBack.DownloadRange(ctx, 0, -1)
	if err != nil {
		log.Fatal("could not initiate download: ", err)
	}
	defer logClose(strm.Close)

	// Read everything from the stream
	_, err = ioutil.ReadAll(strm)
	if err != nil {
		log.Fatal("could not read object: ", err)
	}

	fmt.Println("success")
}
