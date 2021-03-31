// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"
)

type cmdRb struct {
	projectProvider

	force bool

	name string
}

func (c *cmdRb) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.force = f.New("force", "Deletes any objects in bucket first", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.name = a.New("name", "Bucket name (sj://BUCKET)").(string)
}

func (c *cmdRb) Execute(ctx clingy.Context) error {
	return nil
}
