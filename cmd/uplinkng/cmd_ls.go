// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"time"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/storj/cmd/uplinkng/ulloc"
)

type cmdLs struct {
	ex ulext.External

	access    string
	recursive bool
	encrypted bool
	pending   bool
	utc       bool

	prefix *ulloc.Location
}

func newCmdLs(ex ulext.External) *cmdLs {
	return &cmdLs{ex: ex}
}

func (c *cmdLs) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)
	c.recursive = params.Flag("recursive", "List recursively", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.encrypted = params.Flag("encrypted", "Shows keys base64 encoded without decrypting", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.pending = params.Flag("pending", "List pending object uploads instead", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.utc = params.Flag("utc", "Show all timestamps in UTC instead of local time", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.prefix = params.Arg("prefix", "Prefix to list (sj://BUCKET[/KEY])", clingy.Optional,
		clingy.Transform(ulloc.Parse),
	).(*ulloc.Location)
}

func (c *cmdLs) Execute(ctx clingy.Context) error {
	if c.prefix == nil {
		return c.listBuckets(ctx)
	}
	return c.listLocation(ctx, *c.prefix)
}

func (c *cmdLs) listBuckets(ctx clingy.Context) error {
	project, err := c.ex.OpenProject(ctx, c.access)
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()

	tw := newTabbedWriter(ctx.Stdout(), "CREATED", "NAME")
	defer tw.Done()

	iter := project.ListBuckets(ctx, nil)
	for iter.Next() {
		item := iter.Item()
		tw.WriteLine(formatTime(c.utc, item.Created), item.Name)
	}
	return iter.Err()
}

func (c *cmdLs) listLocation(ctx clingy.Context, prefix ulloc.Location) error {
	fs, err := c.ex.OpenFilesystem(ctx, c.access, ulext.BypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	if fs.IsLocalDir(ctx, prefix) {
		prefix = prefix.AsDirectoryish()
	}

	tw := newTabbedWriter(ctx.Stdout(), "KIND", "CREATED", "SIZE", "KEY")
	defer tw.Done()

	// create the object iterator of either existing objects or pending multipart uploads
	var iter ulfs.ObjectIterator
	if c.pending {
		iter, err = fs.ListUploads(ctx, prefix, c.recursive)
	} else {
		iter, err = fs.ListObjects(ctx, prefix, c.recursive)
	}
	if err != nil {
		return err
	}

	// iterate and print the results
	for iter.Next() {
		obj := iter.Item()
		if obj.IsPrefix {
			tw.WriteLine("PRE", "", "", obj.Loc.Loc())
		} else {
			tw.WriteLine("OBJ", formatTime(c.utc, obj.Created), obj.ContentLength, obj.Loc.Loc())
		}
	}
	return iter.Err()
}

func formatTime(utc bool, x time.Time) string {
	if utc {
		x = x.UTC()
	} else {
		x = x.Local()
	}
	return x.Format("2006-01-02 15:04:05")
}
