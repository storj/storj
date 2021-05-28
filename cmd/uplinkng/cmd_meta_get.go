// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
)

type cmdMetaGet struct {
	projectProvider

	path  string
	entry *string
}

func (c *cmdMetaGet) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.path = a.New("path", "Path to object (sj://BUCKET/KEY)").(string)
	c.entry = a.New("entry", "Metadata entry to get", clingy.Optional).(*string)
}

func (c *cmdMetaGet) Execute(ctx clingy.Context) error {
	return nil
}
