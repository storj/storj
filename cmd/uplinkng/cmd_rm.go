// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulloc"
)

type cmdRm struct {
	projectProvider

	recursive bool
	encrypted bool

	location ulloc.Location
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
		clingy.Transform(ulloc.Parse),
	).(ulloc.Location)
}

func (c *cmdRm) Execute(ctx clingy.Context) error {
	fs, err := c.OpenFilesystem(ctx, bypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	if !c.recursive {
		if err := fs.Remove(ctx, c.location); err != nil {
			return err
		}

		fmt.Fprintln(ctx.Stdout(), "removed", c.location)
		return nil
	}

	iter, err := fs.ListObjects(ctx, c.location, c.recursive)
	if err != nil {
		return err
	}

	anyFailed := false
	for iter.Next() {
		loc := iter.Item().Loc

		if err := fs.Remove(ctx, loc); err != nil {
			fmt.Fprintln(ctx.Stderr(), "remove", loc, "failed:", err.Error())
			anyFailed = true
		} else {
			fmt.Fprintln(ctx.Stdout(), "removed", loc)
		}
	}

	if err := iter.Err(); err != nil {
		return errs.Wrap(err)
	} else if anyFailed {
		return errs.New("some removals failed")
	}
	return nil
}
