// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulloc"
	"storj.io/uplink/private/bucket"
)

type cmdRb struct {
	ex ulext.External

	access           string
	force            bool
	bypassGovernance bool

	loc ulloc.Location
}

func newCmdRb(ex ulext.External) *cmdRb {
	return &cmdRb{ex: ex}
}

func (c *cmdRb) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)
	c.force = params.Flag("force", "Deletes any objects in bucket first", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.bypassGovernance = params.Flag("bypass-governance-retention", "Bypass Object Lock governance mode restrictions", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)

	c.loc = params.Arg("name", "Bucket name (sj://BUCKET)",
		clingy.Transform(ulloc.Parse),
	).(ulloc.Location)
}

func (c *cmdRb) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := c.ex.OpenProject(ctx, c.access)
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()

	bucketName, key, ok := c.loc.RemoteParts()
	if !ok {
		return errs.New("location must be remote")
	}
	if key != "" {
		return errs.New("key must not be specified: %q", key)
	}

	if c.force && c.bypassGovernance {
		_, err = bucket.DeleteBucketWithObjectsBypassGovernanceRetention(ctx, project, bucketName)
	} else if c.force {
		_, err = project.DeleteBucketWithObjects(ctx, bucketName)
	} else {
		_, err = project.DeleteBucket(ctx, bucketName)
	}
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Bucket %q has been deleted.\n", bucketName)
	return nil
}
