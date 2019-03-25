// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/storj"
)

// ObjectMeta contains metadata about a specific Object
type ObjectMeta struct {
	Bucket   string
	Path     storj.Path
	IsPrefix bool

	Metadata map[string]string

	Created  time.Time
	Modified time.Time
	Expires  time.Time

	Size     int64
	Checksum []byte
}

// An Object is a sequence of bytes with associated metadata, stored in the
// Storj network (or being prepared for such storage). It belongs to a specific
// bucket, and has a path and a size. It is comparable to a "file" in a
// conventional filesystem.
type Object struct {
	Meta ObjectMeta
}

// DownloadRange returns an object's data. A length of -1 will mean (Object.Size - offset).
func (o *Object) DownloadRange(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	panic("TODO")
}

// Close closes the Object
func (o *Object) Close() error {
	return nil
}
