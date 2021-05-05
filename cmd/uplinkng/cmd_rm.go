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

	location Location
}

func (c *cmdRm) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.recursive = f.New("recursive", "Remove recursively", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.encrypted = f.New("encrypted", "Interprets keys base64 encoded without decrypting", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.location = a.New("location", "Location to remove (sj://BUCKET[/KEY])",
		clingy.Transform(parseLocation),
	).(Location)
}

func (c *cmdRm) Execute(ctx clingy.Context) error {
	fs, err := c.OpenFilesystem(ctx, bypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	// TODO: use the filesystem interface
	// TODO: recursive remove

	// return fs.Delete(ctx, c.location)
	return nil
}
