// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

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
	output    string

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
	c.output = params.Flag("output", "Output Format (tabbed, json)", "tabbed",
		clingy.Short('o'),
	).(string)

	c.prefix = params.Arg("prefix", "Prefix to list (sj://BUCKET[/KEY])", clingy.Optional,
		clingy.Transform(ulloc.Parse),
	).(*ulloc.Location)
}

func (c *cmdLs) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if c.prefix == nil {
		return c.listBuckets(ctx)
	}
	return c.listLocation(ctx, *c.prefix)
}

func (c *cmdLs) listBuckets(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := c.ex.OpenProject(ctx, c.access)
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()

	iter := project.ListBuckets(ctx, nil)

	switch c.output {
	case "tabbed":
		return c.printTabbedBucket(ctx, iter)
	case "json":
		return c.printJSONBucket(ctx, iter)
	default:
		return errs.New("unknown output format, got %s", c.output)
	}
}

func (c *cmdLs) listLocation(ctx context.Context, prefix ulloc.Location) (err error) {
	defer mon.Task()(&ctx)(&err)

	fs, err := c.ex.OpenFilesystem(ctx, c.access, ulext.BypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	if fs.IsLocalDir(ctx, prefix) {
		prefix = prefix.AsDirectoryish()
	}

	// create the object iterator of either existing objects or pending multipart uploads
	iter, err := fs.List(ctx, prefix, &ulfs.ListOptions{
		Recursive: c.recursive,
		Pending:   c.pending,
		Expanded:  c.expanded,
	})
	if err != nil {
		return err
	}

	switch c.output {
	case "tabbed":
		return c.printTabbedLocation(ctx, iter)
	case "json":
		return c.printJSONLocation(ctx, iter)
	default:
		return errs.New("unknown output format, got %s", c.output)
	}
}

func (c *cmdLs) printTabbedBucket(ctx context.Context, iter *uplink.BucketIterator) (err error) {
	tw := newTabbedWriter(clingy.Stdout(ctx), "CREATED", "NAME")
	defer tw.Done()

	for iter.Next() {
		item := iter.Item()
		tw.WriteLine(formatTime(c.utc, item.Created), item.Name)
	}
	return iter.Err()
}

func (c *cmdLs) printJSONBucket(ctx context.Context, iter *uplink.BucketIterator) (err error) {
	jw := json.NewEncoder(clingy.Stdout(ctx))

	for iter.Next() {
		obj := iter.Item()

		err = jw.Encode(obj)
		if err != nil {
			return err
		}
	}
	return iter.Err()
}

func (c *cmdLs) printTabbedLocation(ctx context.Context, iter ulfs.ObjectIterator) (err error) {
	headers := []string{"KIND", "CREATED", "SIZE", "KEY"}
	if c.expanded {
		headers = append(headers, "EXPIRES", "META")
	}

	tw := newTabbedWriter(clingy.Stdout(ctx), headers...)
	defer tw.Done()

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

func (c *cmdLs) printJSONLocation(ctx context.Context, iter ulfs.ObjectIterator) (err error) {
	jw := json.NewEncoder(clingy.Stdout(ctx))

	for iter.Next() {
		obj := iter.Item()

		if obj.IsPrefix {
			err = jw.Encode(struct {
				Kind string `json:"kind"`
				Key  string `json:"key"`
			}{"PRE", obj.Loc.Loc()})
		} else {
			err = jw.Encode(struct {
				Kind     string `json:"kind"`
				Created  string `json:"created"`
				Size     int64  `json:"size"`
				Key      string `json:"key"`
				Expires  string `json:"expires,omitempty"`
				Metadata int    `json:"meta,omitempty"`
			}{"OBJ", formatTime(c.utc, obj.Created), obj.ContentLength, obj.Loc.Loc(), formatTime(c.utc, obj.Expires), sumMetadataSize(obj.Metadata)})
		}
		if err != nil {
			return err
		}
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
