// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulloc"
)

type cmdMetaGet struct {
	ex ulext.External

	access    string
	encrypted bool

	location ulloc.Location
	entry    *string
}

func newCmdMetaGet(ex ulext.External) *cmdMetaGet {
	return &cmdMetaGet{ex: ex}
}

func (c *cmdMetaGet) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)
	c.encrypted = params.Flag("encrypted", "Shows keys base64 encoded without decrypting", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)

	c.location = params.Arg("location", "Location of object (sj://BUCKET/KEY)",
		clingy.Transform(ulloc.Parse),
	).(ulloc.Location)
	c.entry = params.Arg("entry", "Metadata entry to get", clingy.Optional).(*string)
}

func (c *cmdMetaGet) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := c.ex.OpenProject(ctx, c.access, ulext.BypassEncryption(c.encrypted))
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

		_, _ = fmt.Fprintln(clingy.Stdout(ctx), value)
		return nil
	}

	if object.Custom == nil {
		_, _ = fmt.Fprintln(clingy.Stdout(ctx), "{}")
		return nil
	}

	data, err := json.MarshalIndent(object.Custom, "", "  ")
	if err != nil {
		return errs.Wrap(err)
	}

	_, _ = fmt.Fprintln(clingy.Stdout(ctx), string(data))
	return nil
}
