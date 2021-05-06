// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulloc"
)

type cmdMetaGet struct {
	projectProvider

	location ulloc.Location
	entry    *string
}

func (c *cmdMetaGet) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.location = a.New("location", "Location of object (sj://BUCKET/KEY)",
		clingy.Transform(ulloc.Parse),
	).(ulloc.Location)
	c.entry = a.New("entry", "Metadata entry to get", clingy.Optional).(*string)
}

func (c *cmdMetaGet) Execute(ctx clingy.Context) error {
	return nil
}
