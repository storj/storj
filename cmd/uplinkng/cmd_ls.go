// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"time"

	"github.com/zeebo/clingy"
)

type cmdLs struct {
	projectProvider

	recursive bool
	encrypted bool
	pending   bool
	utc       bool

	prefix *Location
}

func (c *cmdLs) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.recursive = f.New("recursive", "List recursively", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.encrypted = f.New("encrypted", "Shows keys base64 encoded without decrypting", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.pending = f.New("pending", "List pending object uploads instead", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.utc = f.New("utc", "Show all timestamps in UTC instead of local time", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.prefix = a.New("prefix", "Prefix to list (sj://BUCKET[/KEY])", clingy.Optional,
		clingy.Transform(parseLocation),
	).(*Location)
}

func (c *cmdLs) Execute(ctx clingy.Context) error {
	if c.prefix == nil {
		return c.listBuckets(ctx)
	}
	return c.listLocation(ctx, *c.prefix)
}

func (c *cmdLs) listBuckets(ctx clingy.Context) error {
	project, err := c.OpenProject(ctx)
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

func (c *cmdLs) listLocation(ctx clingy.Context, prefix Location) error {
	fs, err := c.OpenFilesystem(ctx, bypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	tw := newTabbedWriter(ctx.Stdout(), "KIND", "CREATED", "SIZE", "KEY")
	defer tw.Done()

	// create the object iterator of either existing objects or pending multipart uploads
	var iter objectIterator
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
			tw.WriteLine("PRE", "", "", obj.Loc.Key())
		} else {
			tw.WriteLine("OBJ", formatTime(c.utc, obj.Created), obj.ContentLength, obj.Loc.Key())
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
