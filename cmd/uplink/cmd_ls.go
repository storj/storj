// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"time"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/storj/cmd/uplink/ulloc"
	"storj.io/uplink"
)

type cmdLs struct {
	ex ulext.External

	access    string
	recursive bool
	encrypted bool
	expanded  bool
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
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.encrypted = params.Flag("encrypted", "Shows keys base64 encoded without decrypting", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.pending = params.Flag("pending", "List pending object uploads instead", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.expanded = params.Flag("expanded", "Use expanded output, showing object expiration times and whether there is custom metadata attached", false,
		clingy.Short('x'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.utc = params.Flag("utc", "Show all timestamps in UTC instead of local time", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
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

	headers := []string{"KIND", "CREATED", "SIZE", "KEY"}
	if c.expanded {
		headers = append(headers, "EXPIRES", "META")
	}

	tw := newTabbedWriter(ctx.Stdout(), headers...)
	defer tw.Done()

	// create the object iterator of either existing objects or pending multipart uploads
	iter, err := fs.List(ctx, prefix, &ulfs.ListOptions{
		Recursive: c.recursive,
		Pending:   c.pending,
		Expanded:  c.expanded,
	})
	if err != nil {
		return err
	}

	// iterate and print the results
	for iter.Next() {
		obj := iter.Item()

		var parts []interface{}
		if obj.IsPrefix {
			parts = append(parts, "PRE", "", "", obj.Loc.Loc())
			if c.expanded {
				parts = append(parts, "", "")
			}
		} else {
			parts = append(parts, "OBJ", formatTime(c.utc, obj.Created), obj.ContentLength, obj.Loc.Loc())
			if c.expanded {
				parts = append(parts, formatTime(c.utc, obj.Expires), sumMetadataSize(obj.Metadata))
			}
		}

		tw.WriteLine(parts...)
	}
	return iter.Err()
}

func formatTime(utc bool, x time.Time) string {
	if x.IsZero() {
		return ""
	}

	if utc {
		x = x.UTC()
	} else {
		x = x.Local()
	}
	return x.Format("2006-01-02 15:04:05")
}

func sumMetadataSize(md uplink.CustomMetadata) int {
	size := 0
	for k, v := range md {
		size += len(k)
		size += len(v)
	}
	return size
}
