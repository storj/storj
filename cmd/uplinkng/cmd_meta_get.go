// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulloc"
)

type cmdMetaGet struct {
	projectProvider

	encrypted bool

	location ulloc.Location
	entry    *string
}

func (c *cmdMetaGet) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.encrypted = f.New("encrypted", "Shows keys base64 encoded without decrypting", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.location = a.New("location", "Location of object (sj://BUCKET/KEY)",
		clingy.Transform(ulloc.Parse),
	).(ulloc.Location)
	c.entry = a.New("entry", "Metadata entry to get", clingy.Optional).(*string)
}

func (c *cmdMetaGet) Execute(ctx clingy.Context) error {
	project, err := c.OpenProject(ctx, bypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()

	bucket, key, ok := c.location.RemoteParts()
	if !ok {
		return errs.New("location must be remote")
	}

	object, err := project.StatObject(ctx, bucket, key)
	if err != nil {
		return err
	}

	if c.entry != nil {
		value, ok := object.Custom[*c.entry]
		if !ok {
			return errs.New("entry %q does not exist", *c.entry)
		}

		fmt.Fprintln(ctx.Stdout(), value)
		return nil
	}

	if object.Custom == nil {
		fmt.Fprintln(ctx.Stdout(), "{}")
		return nil
	}

	data, err := json.MarshalIndent(object.Custom, "", "  ")
	if err != nil {
		return errs.Wrap(err)
	}

	fmt.Fprintln(ctx.Stdout(), string(data))
	return nil
}
