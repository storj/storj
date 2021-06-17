// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type cmdMb struct {
	projectProvider

	name string
}

func (c *cmdMb) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.name = a.New("name", "Bucket name (sj://BUCKET)").(string)
}

func (c *cmdMb) Execute(ctx clingy.Context) error {
	project, err := c.OpenProject(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = project.Close() }()

	_, err = project.CreateBucket(ctx, c.name)
	return err
}
