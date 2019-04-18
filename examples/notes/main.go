// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// notes application demonstrates how to use Storj for storing notes.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

var (
	satellite  = flag.String("satellite", "mars.tardigrade.io:7777", "satellite that store meta data")
	apiKeyText = flag.String("api-key", os.Getenv("NOTES_API_KEY"), "api key for connecting to the satellite")
	bucketName = flag.String("bucket", "notes", "bucket to store notes in")
	password   = flag.String("password", os.Getenv("NOTES_PASSWORD"), "password for accessing data")
)

func main() {
	flag.Parse()
	if *apiKeyText == "" {
		fatalf("api key is missing\n")
	}
	if *password == "" {
		fatalf("encryption key is missing\n")
	}
	apiKey, err := uplink.ParseAPIKey(*apiKeyText)
	if err != nil {
		fatalf("unable to parse API Key: %v", err)
	}

	ctx := context.Background()

	notes, err := OpenNotes(ctx, apiKey, *password)
	if err != nil {
		fatalf("unable to open notes: %v", err)
	}
	defer notes.Close()

	command, key, value := flag.Arg(0), flag.Arg(1), flag.Arg(2)

	switch command {
	case "put":
		err := notes.Put(ctx, key, value)
		if err != nil {
			fatalf("unable to put %q: %v", key, err)
		}

	case "get":
		value, err := notes.Get(ctx, key)
		if err != nil {
			fatalf("unable to get %q: %v", key, err)
		}
		fmt.Println(value)

	case "list":
		keys, err := notes.List(ctx, key)
		if err != nil {
			fatalf("unable to list %q: %v", key, err)
		}
		for _, key := range keys {
			fmt.Println(key)
		}

	case "help":
		flag.Usage()

	default:
		flag.Usage()
		os.Exit(1)
	}
}

type Notes struct {
	Client  *uplink.Uplink
	Project *uplink.Project
	Bucket  *uplink.Bucket
}

func OpenNotes(ctx context.Context, apiKey uplink.APIKey, password string) (*Notes, error) {
	// Create an uplink for communicating with the peers.
	client, err := uplink.NewUplink(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create uplink: %v", err)
	}

	// figure out the encryption key
	encryptionKey := deriveEncryptionKey(password)

	options := &uplink.ProjectOptions{}
	options.Volatile.EncryptionKey = &encryptionKey

	// open the project
	project, err := client.OpenProject(ctx, *satellite, apiKey, options)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("unable to open project: %v\n", err)
	}

	// create a bucket for storing data
	_, err = project.CreateBucket(ctx, *bucketName, nil)
	if err != nil { // TODO: check whether bucket already exists
		project.Close()
		client.Close()
		return nil, fmt.Errorf("unable to create bucket: %v\n", err)
	}

	// Open up the desired Bucket within the Project
	bucket, err := project.OpenBucket(ctx, *bucketName, &uplink.EncryptionAccess{Key: encryptionKey})
	if err != nil {
		project.Close()
		client.Close()
		return nil, fmt.Errorf("could not open bucket: %v\n", err)
	}

	return &Notes{
		Client:  client,
		Project: project,
		Bucket:  bucket,
	}, nil
}

func (notes *Notes) Close() error {
	bucketErr := notes.Bucket.Close()
	projectErr := notes.Project.Close()
	clientErr := notes.Client.Close()

	if bucketErr != nil || projectErr != nil || clientErr != nil {
		return fmt.Errorf("error occurred while closing: %v %v %v", bucketErr, projectErr, clientErr)
	}
	return nil
}

func (notes *Notes) Put(ctx context.Context, key, value string) error {
	// create a io.Reader for uploading
	buffer := bytes.NewBuffer([]byte(value))
	// upload all the data
	err := notes.Bucket.UploadObject(ctx, key, buffer, nil)
	if err != nil {
		return fmt.Errorf("unable to upload: %v", err)
	}
	return nil
}

func (notes *Notes) Get(ctx context.Context, key string) (string, error) {
	// open the object we want to download
	object, err := notes.Bucket.OpenObject(ctx, key)
	if err != nil {
		return "", fmt.Errorf("unable to open object: %v", err)
	}
	defer object.Close()

	// we want the whole thing, so range from 0 to -1
	stream, err := object.DownloadRange(ctx, 0, -1)
	if err != nil {
		return "", fmt.Errorf("unable to start downloading: %v", err)
	}
	defer stream.Close()

	// read everything from the stream
	data, err := ioutil.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("download failed: %v", err)
	}

	return string(data), nil
}

func (notes *Notes) List(ctx context.Context, prefix string) ([]string, error) {

}

func deriveEncryptionKey(password string) storj.Key {
	return nil
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
