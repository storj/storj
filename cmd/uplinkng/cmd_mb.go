// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
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
	c.access = params.Flag("access", "Which access to use", "").(string)

	c.name = params.Arg("name", "Bucket name (sj://BUCKET)").(string)
}

func (c *cmdMb) Execute(ctx clingy.Context) error {
	project, err := c.ex.OpenProject(ctx, c.access)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = project.Close() }()

	_, err = project.CreateBucket(ctx, c.name)
	return err
}
