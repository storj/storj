// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"
)

type cmdRm struct {
	projectProvider

	recursive bool
	encrypted bool

	path string
}

func (c *cmdRm) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.recursive = f.New("recursive", "List recursively", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.encrypted = f.New("encrypted", "Shows paths as base64-encoded encrypted paths", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.path = a.New("path", "Path to remove (sj://BUCKET[/KEY])").(string)
}

func (c *cmdRm) Execute(ctx clingy.Context) error {
	return nil
}
