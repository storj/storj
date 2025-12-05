// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulloc"
)

type cmdMb struct {
	ex ulext.External

	access string

	name string
}

func newCmdMb(ex ulext.External) *cmdMb {
	return &cmdMb{ex: ex}
}

func (c *cmdMb) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)

	c.name = params.Arg("name", "Bucket name (sj://BUCKET)", clingy.Transform(ulloc.Parse),
		clingy.Transform(func(location ulloc.Location) (string, error) {
			if bucket, key, ok := location.RemoteParts(); key == "" && ok {
				return bucket, nil
			}
			return "", errs.New("invalid bucket name")
		}),
	).(string)
}

func (c *cmdMb) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := c.ex.OpenProject(ctx, c.access)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = project.Close() }()

	_, err = project.CreateBucket(ctx, c.name)
	return err
}
