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
	Expiration time.Time
	Data       []byte
	// ObjectInfo - represents object metadata.
	// Name of the bucket.
	Bucket string

	// Name of the object.
	Name string

	// Date and time when the object was last modified.
	ModTime time.Time

	// Total object size.
	Size int64

	// IsDir indicates if the object is prefix.
	IsDir bool

	// Hex encoded unique entity tag of the object.
	ETag string

	// A standard MIME type describing the format of the object.
	ContentType string

	// Specifies what content encodings have been applied to the object and thus
	// what decoding mechanisms must be applied to obtain the object referenced
	// by the Content-Type header field.
	ContentEncoding string

	// Specify object storage class
	StorageClass string

	// User-Defined metadata
	UserDefined map[string]string

	// Date and time when the object was last accessed.
	AccTime time.Time
}

//PutObject interface method
func (o *Objects) PutObject(ctx context.Context, objpath string, data io.Reader, metadata []byte, expiration time.Time) (size int64, err error) {
	defer mon.Task()(&ctx)(&err)
	return 0, nil
}

//GetObject interface method
func (o *Objects) GetObject(ctx context.Context, objpath string) (r ranger.Ranger, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	m = Meta{
		Name: "GetObject",
	}
	return nil, m, nil
}

//DeleteObject interface method
func (o *Objects) DeleteObject(ctx context.Context, objpath string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

//ListObjects interface method
func (o *Objects) ListObjects(ctx context.Context, startingPath, endingPath string) (objpaths []string, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	objpaths = []string{"objpath1", "objpath2", "objpath3"}
	truncated = true
	err = nil
	return objpaths, truncated, err
}

//SetXAttr interface method
func (o *Objects) SetXAttr(ctx context.Context, objpath, xattr string, data io.Reader, metadata []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

//GetXAttr interface method
func (o *Objects) GetXAttr(ctx context.Context, objpath, xattr string) (r ranger.Ranger, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	m = Meta{
		Name: "GetXAttr",
	}
	return nil, m, nil
}

//DeleteXAttr interface method
func (o *Objects) DeleteXAttr(ctx context.Context, path, xattr string) (err error) {
	defer mon.Task()(&ctx)(&err)
	return nil
}

//ListXAttrs interface method
func (o *Objects) ListXAttrs(ctx context.Context, path, startingXAttr, endingXAttr string) (xattrs []string, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	xattrs = []string{"xattrs1", "xattrs2", "xattrs3"}
	truncated = true
	err = nil
	return xattrs, truncated, err
}
