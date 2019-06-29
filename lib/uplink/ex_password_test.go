// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/lib/uplink"
)

func CreateEncryptionKeyExampleByAdmin1(ctx context.Context,
	satelliteAddress, apiKey string, cfg *uplink.Config, out io.Writer) (
	serializedAccess string, err error) {

	errCatch := func(fn func() error) { err = errs.Combine(err, fn()) }

	// First, create an Uplink handle.
	ul, err := uplink.NewUplink(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer errCatch(ul.Close)

	// Parse the API key. API keys are "macaroons" that allow you to create new,
	// restricted API keys.
	key, err := uplink.ParseAPIKey(apiKey)
	if err != nil {
		return "", err
	}

	// Open the project in question. Projects are identified by a specific
	// Satellite and API key
	p, err := ul.OpenProject(ctx, satelliteAddress, key)
	if err != nil {
		return "", err
	}
	defer errCatch(p.Close)

	// Make a key
	encKey, err := p.SaltedKeyFromPassphrase(ctx, "my secret passphrase")
	if err != nil {
		return "", err
	}

	// Make an encryption context
	access := uplink.NewEncryptionAccessWithDefaultKey(*encKey)
	// serialize it
	serializedAccess, err = access.Serialize()
	if err != nil {
		return "", err
	}

	// Create a bucket
	_, err = p.CreateBucket(ctx, "prod", nil)
	if err != nil {
		return "", err
	}

	// Open bucket
	bucket, err := p.OpenBucket(ctx, "prod", access)
	if err != nil {
		return "", err
	}
	defer errCatch(bucket.Close)

	// Upload a file
	err = bucket.UploadObject(ctx, "webserver/logs/log.txt",
		strings.NewReader("hello world"), nil)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(out, "success!")
	return serializedAccess, nil
}

func CreateEncryptionKeyExampleByAdmin2(ctx context.Context,
	satelliteAddress, apiKey string, serializedAccess string,
	cfg *uplink.Config, out io.Writer) (err error) {

	errCatch := func(fn func() error) { err = errs.Combine(err, fn()) }

	// First, create an Uplink handle.
	ul, err := uplink.NewUplink(ctx, cfg)
	if err != nil {
		return err
	}
	defer errCatch(ul.Close)

	// Parse the API key. API keys are "macaroons" that allow you to create new,
	// restricted API keys.
	key, err := uplink.ParseAPIKey(apiKey)
	if err != nil {
		return err
	}

	// Open the project in question. Projects are identified by a specific
	// Satellite and API key
	p, err := ul.OpenProject(ctx, satelliteAddress, key)
	if err != nil {
		return err
	}
	defer errCatch(p.Close)

	// Parse the encryption context
	access, err := uplink.ParseEncryptionAccess(serializedAccess)
	if err != nil {
		return err
	}

	// Open bucket
	bucket, err := p.OpenBucket(ctx, "prod", access)
	if err != nil {
		return err
	}
	defer errCatch(bucket.Close)

	// Open file
	obj, err := bucket.OpenObject(ctx, "webserver/logs/log.txt")
	if err != nil {
		return err
	}
	defer errCatch(obj.Close)

	// Get a reader for the entire file
	r, err := obj.DownloadRange(ctx, 0, -1)
	if err != nil {
		return err
	}
	defer errCatch(r.Close)

	// Read the file
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	// Print it!
	fmt.Fprintln(out, string(data))
	return nil
}

func Example_createEncryptionKey() {
	// The satellite address is the address of the satellite your API key is
	// valid on
	satelliteAddress := "us-central-1.tardigrade.io:7777"

	// The API key can be created in the web interface
	admin1APIKey := "qPSUM3k0bZyOIyil2xrVWiSuc9HuB2yBP3qDrA2Gc"
	admin2APIKey := "udP0lzCC2rgwRZfdY70PcwWrXzrq9cl5usbiFaeyo"

	ctx := context.Background()

	// Admin1 is going to create an encryption context and share it
	access, err := CreateEncryptionKeyExampleByAdmin1(ctx, satelliteAddress,
		admin1APIKey, &uplink.Config{}, os.Stdout)
	if err != nil {
		panic(err)
	}

	// Admin2 is going to use the provided encryption context to load the
	// uploaded file
	err = CreateEncryptionKeyExampleByAdmin2(ctx, satelliteAddress,
		admin2APIKey, access, &uplink.Config{}, os.Stdout)
	if err != nil {
		panic(err)
	}
}
