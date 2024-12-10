// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulloc"
)

type cmdGet struct {
	access   *AccessOptions
	location string

	bucket string
	key    string
}

func newCmdGet() *cmdGet {
	return &cmdGet{
		access: newAccessOptions(),
	}
}

func (c *cmdGet) Setup(params clingy.Parameters) {
	c.access.Setup(params)
	c.location = params.Arg("location", "Location of object (sj://BUCKET/KEY)").(string)
}

func (c *cmdGet) Validate() (err error) {
	err = c.access.Validate()
	if err != nil {
		return err
	}

	loc, err := ulloc.Parse(c.location)
	if err != nil {
		return fmt.Errorf("invalid location '%s': %w", c.location, err)
	}

	var ok bool
	c.bucket, c.key, ok = loc.RemoteParts()
	if !ok {
		return fmt.Errorf("invalid location '%s': must be remote", c.location)
	}

	if c.bucket == "" || c.key == "" {
		return fmt.Errorf("invalid location '%s': both bucket and key must be provided", c.location)
	}

	return nil
}

func (c *cmdGet) Execute(ctx context.Context) (err error) {
	err = c.Validate()
	if err != nil {
		return err
	}

	client := newMetaSearchClient(c.access)
	meta, err := client.GetObjectMetadata(ctx, c.bucket, c.key)
	if err != nil {
		return fmt.Errorf("cannot format metadata: %w", err)
	}

	formattedMeta, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot format metadata: %w", err)
	}

	fmt.Println(string(formattedMeta))
	return nil
}
