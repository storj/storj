// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulloc"
)

type cmdSet struct {
	access   *AccessOptions
	location string

	bucket string
	key    string

	inputfile string
	inputdata string

	metadata map[string]interface{}
}

func newCmdSet() *cmdSet {
	return &cmdSet{
		access: newAccessOptions(),
	}
}

func (c *cmdSet) Setup(params clingy.Parameters) {
	c.access.Setup(params)
	c.inputfile = params.Flag("input-file", "File containing metadata to set", "", clingy.Short('i')).(string)
	c.inputdata = params.Flag("data", "Metadata to set", "", clingy.Short('d')).(string)

	c.location = params.Arg("location", "Location of object (sj://BUCKET/KEY)").(string)
}

func (c *cmdSet) Validate() (err error) {
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

	if c.inputfile == "" && c.inputdata == "" {
		return fmt.Errorf("either --input-file or --data must be provided")
	}

	return nil
}

func (c *cmdSet) Execute(ctx context.Context) (err error) {
	err = c.Validate()
	if err != nil {
		return err
	}

	err = c.setMetadata()
	if err != nil {
		return err
	}

	client := newMetaSearchClient(c.access)
	err = client.SetObjectMetadata(ctx, c.bucket, c.key, c.metadata)
	if err != nil {
		return fmt.Errorf("cannot set metadata: %w", err)
	}

	return nil
}

func (c *cmdSet) setMetadata() (err error) {
	var inputdata []byte

	if c.inputfile == "-" {
		inputdata, err = io.ReadAll(os.Stdin)
	} else if c.inputfile != "" {
		inputdata, err = os.ReadFile(c.inputfile)
	} else {
		inputdata = []byte(c.inputdata)
	}

	if err != nil {
		return fmt.Errorf("error reading metadata: %w", err)
	}

	err = json.Unmarshal([]byte(inputdata), &c.metadata)
	if err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}
	return nil
}
