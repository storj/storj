// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/uplink"
)

type cmdLs struct {
	projectProvider

	recursive bool
	encrypted bool
	pending   bool

	prefix *string
}

func (c *cmdLs) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.recursive = f.New("recursive", "List recursively", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.encrypted = f.New("encrypted", "Shows paths as base64-encoded encrypted paths", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.pending = f.New("pending", "List pending multipart object uploads instead", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.prefix = a.New("prefix", "Prefix to list (sj://BUCKET[/KEY])", clingy.Optional).(*string)
}

func (c *cmdLs) Execute(ctx clingy.Context) error {
	if c.prefix == nil {
		return c.listBuckets(ctx)
	}
	return c.listPath(ctx, *c.prefix)
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
		tw.WriteLine(formatTime(item.Created), item.Name)
	}
	return iter.Err()
}

func (c *cmdLs) listPath(ctx clingy.Context, path string) error {
	bucket, key, ok, err := parsePath(path)
	if err != nil {
		return err
	} else if !ok {
		return errs.New("no bucket specified. use format sj://bucket")
	}

	project, err := c.OpenProject(ctx, bypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()

	tw := newTabbedWriter(ctx.Stdout(), "KIND", "CREATED", "SIZE", "KEY")
	defer tw.Done()

	// in order to get a correct listing, including non-terminating components, what we
	// must do is pop the last component off, ensuring the prefix is either empty or
	// ends with a /, list there, then filter the results locally against the popped component.
	prefix, filter := "", key
	if idx := strings.LastIndexByte(key, '/'); idx >= 0 {
		prefix, filter = key[:idx+1], key[idx+1:]
	}

	// create the object iterator of either existing objects or pending multipart uploads
	var iter listObjectIterator
	if c.pending {
		iter = (*uplinkUploadIterator)(project.ListUploads(ctx, bucket,
			&uplink.ListUploadsOptions{
				Prefix:    prefix,
				Recursive: c.recursive,
				System:    true,
			}))
	} else {
		iter = (*uplinkObjectIterator)(project.ListObjects(ctx, bucket,
			&uplink.ListObjectsOptions{
				Prefix:    prefix,
				Recursive: c.recursive,
				System:    true,
			}))
	}

	// iterate and print the results
	for iter.Next() {
		obj := iter.Item()
		key := obj.Key[len(prefix):]

		if !strings.HasPrefix(key, filter) {
			continue
		}

		if obj.IsPrefix {
			tw.WriteLine("PRE", "", "", key)
		} else {
			tw.WriteLine("OBJ", formatTime(obj.Created), obj.ContentLength, key)
		}
	}
	return iter.Err()
}

func formatTime(x time.Time) string {
	return x.Local().Format("2006-01-02 15:04:05")
}

// the following code wraps the two list iterator types behind an interface so that
// the list code can be generic against either of them.

type listObjectIterator interface {
	Next() bool
	Err() error
	Item() listObject
}

type listObject struct {
	Key           string
	IsPrefix      bool
	Created       time.Time
	ContentLength int64
}

type uplinkObjectIterator uplink.ObjectIterator

func (u *uplinkObjectIterator) Next() bool { return (*uplink.ObjectIterator)(u).Next() }
func (u *uplinkObjectIterator) Err() error { return (*uplink.ObjectIterator)(u).Err() }
func (u *uplinkObjectIterator) Item() listObject {
	obj := (*uplink.ObjectIterator)(u).Item()
	return listObject{
		Key:           obj.Key,
		IsPrefix:      obj.IsPrefix,
		Created:       obj.System.Created,
		ContentLength: obj.System.ContentLength,
	}
}

type uplinkUploadIterator uplink.UploadIterator

func (u *uplinkUploadIterator) Next() bool { return (*uplink.UploadIterator)(u).Next() }
func (u *uplinkUploadIterator) Err() error { return (*uplink.UploadIterator)(u).Err() }
func (u *uplinkUploadIterator) Item() listObject {
	obj := (*uplink.UploadIterator)(u).Item()
	return listObject{
		Key:           obj.Key,
		IsPrefix:      obj.IsPrefix,
		Created:       obj.System.Created,
		ContentLength: obj.System.ContentLength,
	}
}
