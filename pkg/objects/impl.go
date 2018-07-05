// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/paths"
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

//PutObject interface method
func (o *Objects) PutObject(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

//GetObject interface method
func (o *Objects) GetObject(ctx context.Context, path paths.Path) (rr ranger.RangeCloser, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	m = Meta{}
	rr = nil
	err = nil
	return rr, m, err
}

//DeleteObject interface method
func (o *Objects) DeleteObject(ctx context.Context, path paths.Path) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = nil
	return err
}

//ListObjects interface method
func (o *Objects) ListObjects(ctx context.Context, startingPath, endingPath paths.Path) (path []paths.Path, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	path = []paths.Path{{"x"}, {"objpath1", "objpath2", "objpath3"}}
	truncated = true
	err = nil
	return path, truncated, err
}

//SetXAttr interface method
func (o *Objects) SetXAttr(ctx context.Context, path paths.Path, xattr string, data io.Reader, metadata []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

//GetXAttr interface method
func (o *Objects) GetXAttr(ctx context.Context, path paths.Path, xattr string) (rr ranger.RangeCloser, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	m = Meta{}
	rr = nil
	err = nil
	return rr, m, err
}

//DeleteXAttr interface method
func (o *Objects) DeleteXAttr(ctx context.Context, path paths.Path, xattr string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

//ListXAttrs interface method
func (o *Objects) ListXAttrs(ctx context.Context, path paths.Path, startingXAttr, endingXAttr string) (xattrs []string, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	xattrs = []string{"xattrs1", "xattrs2", "xattrs3"}
	truncated = true
	err = nil
	return xattrs, truncated, err
}
