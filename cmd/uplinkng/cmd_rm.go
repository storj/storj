// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type cmdRm struct {
	projectProvider

	recursive bool
	encrypted bool

	location string
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

	c.location = a.New("location", "Location to remove (sj://BUCKET[/KEY])").(string)
}

func (c *cmdRm) Execute(ctx clingy.Context) error {
	project, err := c.OpenProject(ctx, bypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()

	// TODO: use the filesystem interface
	// TODO: recursive remove

	p, err := parseLocation(c.location)
	if err != nil {
		return err
	} else if !p.remote {
		return errs.New("can only delete remote objects")
	}

	_, err = project.DeleteObject(ctx, p.bucket, p.key)
	return err
}
