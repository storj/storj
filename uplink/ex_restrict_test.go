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

	"storj.io/common/macaroon"
	"storj.io/storj/lib/uplink"
)

func RestrictAccessExampleByAdmin(ctx context.Context,
	satelliteAddress, apiKey, adminAccess string,
	cfg *uplink.Config, out io.Writer) (
	serializedScope string, err error) {

	// Parse the API key. API keys are "macaroons" that allow you to create new,
	// restricted API keys.
	key, err := uplink.ParseAPIKey(apiKey)
	if err != nil {
		return "", err
	}

	// Restrict the API key to be read only and to be for just the prod and
	// staging buckets for the path webserver/logs/
	userAPIKey, err := key.Restrict(macaroon.Caveat{
		DisallowWrites:  true,
		DisallowDeletes: true,
	})
	if err != nil {
		return "", err
	}

	// Load the existing encryption access context
	access, err := uplink.ParseEncryptionAccess(adminAccess)
	if err != nil {
		return "", err
	}

	// Restrict the encryption access context to just the prod and staging
	// buckets for webserver/logs/
	userAPIKey, userAccess, err := access.Restrict(userAPIKey,
		uplink.EncryptionRestriction{
			Bucket:     "prod",
			PathPrefix: "webserver/logs",
		},
		uplink.EncryptionRestriction{
			Bucket:     "staging",
			PathPrefix: "webserver/logs",
		},
	)
	if err != nil {
		return "", err
	}

	userScope := &uplink.Scope{
		SatelliteAddr:    satelliteAddress,
		APIKey:           userAPIKey,
		EncryptionAccess: userAccess,
	}

	// Serialize the scope
	serializedScope, err = userScope.Serialize()
	if err != nil {
		return "", err
	}

	fmt.Fprintln(out, "success!")
	return serializedScope, nil
}

func RestrictAccessExampleByUser(ctx context.Context,
	serializedScope string, cfg *uplink.Config, out io.Writer) (err error) {
	errCatch := func(fn func() error) { err = errs.Combine(err, fn()) }

	// First, create an Uplink handle.
	ul, err := uplink.NewUplink(ctx, cfg)
	if err != nil {
		return err
	}
	defer errCatch(ul.Close)

	// Parse the scope.
	scope, err := uplink.ParseScope(serializedScope)
	if err != nil {
		return err
	}

	// Open the project in question. Projects are identified by a specific
	// Satellite and API key
	p, err := ul.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
	if err != nil {
		return err
	}
	defer errCatch(p.Close)

	// Open bucket
	bucket, err := p.OpenBucket(ctx, "prod", scope.EncryptionAccess)
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
	// The satellite address is the address of the satellite your API key is
	// valid on
	satelliteAddress := "us-central-1.tardigrade.io:7777"

	// The API key can be created in the web interface
	adminAPIKey := "qPSUM3k0bZyOIyil2xrVWiSuc9HuB2yBP3qDrA2Gc"

	// The encryption access context was created using
	// NewEncryptionAccessWithDefaultKey and
	// (*Project).SaltedKeyFromPassphrase() earlier
	adminAccess := "HYGoqCEz43mCE40Hc5lQD3DtUYynx9Vo1GjOx75hQ"

	ctx := context.Background()

	// Admin1 is going to create a scope and share it
	userScope, err := RestrictAccessExampleByAdmin(ctx, satelliteAddress,
		adminAPIKey, adminAccess, &uplink.Config{}, os.Stdout)
	if err != nil {
		panic(err)
	}

	// Admin2 is going to use the provided scope to load the uploaded file
	err = RestrictAccessExampleByUser(ctx, userScope, &uplink.Config{}, os.Stdout)
	if err != nil {
		panic(err)
	}
}
