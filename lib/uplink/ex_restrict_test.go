// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/macaroon"
)

func RestrictAccessExample_Admin(ctx context.Context, satelliteAddress, apiKey string, adminEncCtx string, cfg *uplink.Config, out io.Writer) (userAPIKey_ string, serializedEncCtx_ []byte, err error) {
	// Parse the API key. API keys are "macaroons" that allow you to create new, restricted API keys.
	key, err := uplink.ParseAPIKey(apiKey)
	if err != nil {
		return "", nil, err
	}

	// Restrict the API key to be read only and to be for just the prod and staging buckets
	// for the path webserver/logs/
	userAPIKey, err := key.Restrict(macaroon.Caveat{
		DisallowWrites:  true,
		DisallowDeletes: true,
	})
	if err != nil {
		return "", nil, err
	}

	// Load the existing encryption context
	encCtx, err := uplink.ParseEncryptionCtx([]byte(adminEncCtx))
	if err != nil {
		return "", nil, err
	}

	// Restrict the encryption context to just the prod and staging buckets
	// for webserver/logs/
	userAPIKey, userEncCtx, err := encCtx.Restrict(userAPIKey,
		uplink.EncryptionRestriction{Bucket: "prod", PathPrefix: "webserver/logs/"},
		uplink.EncryptionRestriction{Bucket: "staging", PathPrefix: "webserver/logs/"},
	)
	if err != nil {
		return "", nil, err
	}

	// Serialize the encryption context
	serializedUserEncCtx, err := userEncCtx.Serialize()
	if err != nil {
		return "", nil, err
	}

	return userAPIKey.Serialize(), serializedUserEncCtx, nil
}

func RestrictAccessExample_User(ctx context.Context, satelliteAddress, apiKey string, serializedEncCtx []byte, cfg *uplink.Config, out io.Writer) (err error) {
	errCatch := func(fn func() error) { err = errs.Combine(err, fn()) }

	// First, create an Uplink handle.
	ul, err := uplink.NewUplink(ctx, cfg)
	if err != nil {
		return err
	}
	defer errCatch(ul.Close)

	// Parse the API key.
	key, err := uplink.ParseAPIKey(apiKey)
	if err != nil {
		return err
	}

	// Open the project in question. Projects are identified by a specific Satellite and API key
	p, err := ul.OpenProject(ctx, satelliteAddress, key)
	if err != nil {
		return err
	}
	defer errCatch(p.Close)

	// Parse the encryption context
	encCtx, err := uplink.ParseEncryptionCtx(serializedEncCtx)
	if err != nil {
		return err
	}

	// Open bucket
	bucket, err := p.OpenBucket(ctx, "prod", encCtx)
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

func Example_restrictAccess() {
	// The satellite address is the address of the satellite your API key is valid on
	satelliteAddress := "us-central-1.tardigrade.io:7777"

	// The API key can be created in the web interface
	adminAPIKey := "qPSUM3k0bZyOIyil2xrVWiSuc9HuB2yBP3qDrA2Gc"

	// The encryption context was created using NewEncryptionCtxWithDefaultKey and
	// (*Project).SaltedKeyFromPassphrase() earlier
	adminEncCtx := "HYGoqCEz43mCE40Hc5lQD3DtUYynx9Vo1GjOx75hQ"

	ctx := context.Background()

	// Admin1 is going to create an encryption context and share it
	userAPIKey, encCtx, err := RestrictAccessExample_Admin(ctx, satelliteAddress, adminAPIKey, adminEncCtx, &uplink.Config{}, os.Stdout)
	if err != nil {
		panic(err)
	}

	// Admin2 is going to use the provided encryption context to load the uploaded file
	err = RestrictAccessExample_User(ctx, satelliteAddress, userAPIKey, encCtx, &uplink.Config{}, os.Stdout)
	if err != nil {
		panic(err)
	}
}
