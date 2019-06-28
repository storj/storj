// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/lib/uplink"
)

func CreateBucketExample(ctx context.Context, satelliteAddress string, apiKey string, cfg *uplink.Config, out io.Writer) (err error) {
	errCatch := func(fn func() error) { err = errs.Combine(err, fn()) }

	// First, create an Uplink handle.
	ul, err := uplink.NewUplink(ctx, cfg)
	if err != nil {
		return err
	}
	defer errCatch(ul.Close)

	// Then, parse the API key. API keys are "macaroons" that allow you to create new, restricted
	// API keys.
	key, err := uplink.ParseAPIKey(apiKey)
	if err != nil {
		return err
	}

	// Next, open the project in question. Projects are identified by a specific Satellite and API key
	p, err := ul.OpenProject(ctx, satelliteAddress, key)
	if err != nil {
		return err
	}
	defer errCatch(p.Close)

	// Last, create the bucket!
	_, err = p.CreateBucket(ctx, "testbucket", nil)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "success!")
	return nil
}

func Example_createBucket() {
	// The satellite address is the address of the satellite your API key is valid on
	satelliteAddress := "us-central-1.tardigrade.io:7777"

	// The API key can be created in the web interface
	apiKey := "qPSUM3k0bZyOIyil2xrVWiSuc9HuB2yBP3qDrA2Gc"

	err := CreateBucketExample(context.Background(), satelliteAddress, apiKey, &uplink.Config{}, os.Stdout)
	if err != nil {
		panic(err)
	}
}
