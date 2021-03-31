// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type cmdLs struct {
	projectProvider

	recursive bool
	encrypted bool

	path *string
}

func (c *cmdLs) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.recursive = f.New("recursive", "List recursively", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.encrypted = f.New("encrypted", "Shows paths as base64-encoded encrypted paths", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.path = a.New("path", "Path to list (sj://BUCKET[/KEY])", clingy.Optional).(*string)
}

func (c *cmdLs) Execute(ctx clingy.Context) error {
	if c.path == nil {
		return c.listBuckets(ctx)
	}
	return c.listPath(ctx, *c.path)
}

func (c *cmdLs) listBuckets(ctx clingy.Context) error {
	project, err := c.OpenProject(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()

	iter := project.ListBuckets(ctx, nil)
	for iter.Next() {
		item := iter.Item()
		fmt.Fprintln(ctx, "BKT", item.Created.Local().Format("2006-01-02 15:04:05"), item.Name)
	}
	return iter.Err()
}

func (c *cmdLs) listPath(ctx clingy.Context, path string) error {
	return errs.New("TODO")
}
