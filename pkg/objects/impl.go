// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/ranger"
)

//Objects structure
type Objects struct {
}

//Meta structure
type Meta struct {
	Modified   time.Time
	Expiration time.Time
	Data       []byte
}

// func NewObjects(store streams.StreamStore) ObjectStore {
// 	panic("TODO")
// }

//PutObject interface method
func (o *Objects) PutObject(ctx context.Context, path string, data io.Reader, metadata []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//GetObject interface method
func (o *Objects) GetObject(ctx context.Context, path string) (r ranger.Ranger, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//DeleteObject interface method
func (o *Objects) DeleteObject(ctx context.Context, path string) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//ListObjects interface method
func (o *Objects) ListObjects(ctx context.Context, startingPath, endingPath string) (paths []string, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//SetXAttr interface method
func (o *Objects) SetXAttr(ctx context.Context, path, xattr string, data io.Reader, metadata []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//GetXAttr interface method
func (o *Objects) GetXAttr(ctx context.Context, path, xattr string) (r ranger.Ranger, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//DeleteXAttr interface method
func (o *Objects) DeleteXAttr(ctx context.Context, path, xattr string) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//ListXAttrs interface method
func (o *Objects) ListXAttrs(ctx context.Context, path, startingXAttr, endingXAttr string) (xattrs []string, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}
