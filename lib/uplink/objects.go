// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/storj"
)

// Object holds the information for a given object
type Object struct {
	Meta ObjectMeta
}

// ObjectMeta represents metadata about a specific Object
type ObjectMeta struct {
	Bucket   string
	Path     storj.Path
	IsPrefix bool

	Metadata map[string]string

	Created  time.Time
	Modified time.Time
	Expires  time.Time

	Size     int64
	Checksum string

	// this differs from storj.Object by not having Version (yet), and not
	// having a Stream embedded. I'm also not sold on splitting ContentType out
	// from Metadata but shrugemoji.
}

// UploadOpts controls options about uploading a new Object, if authorized.
type UploadOpts struct {
	Metadata map[string]string
	Expires  time.Time

	EncryptionScheme *storj.EncryptionScheme
}

// ListObjectsField numbers the fields of list objects
type ListObjectsField int

const (
	// ListObjectsMetaNone opts
	ListObjectsMetaNone ListObjectsField = 0
	// ListObjectsMetaModified opts
	ListObjectsMetaModified ListObjectsField = 1 << iota
	// ListObjectsMetaExpiration opts
	ListObjectsMetaExpiration ListObjectsField = 1 << iota
	// ListObjectsMetaSize opts
	ListObjectsMetaSize ListObjectsField = 1 << iota
	// ListObjectsMetaChecksum opts
	ListObjectsMetaChecksum ListObjectsField = 1 << iota
	// ListObjectsMetaUserDefined opts
	ListObjectsMetaUserDefined ListObjectsField = 1 << iota
	// ListObjectsMetaAll opts
	ListObjectsMetaAll ListObjectsField = 1 << iota
)

// ListObjectsConfig holds params for listing objects with the Gateway
type ListObjectsConfig struct {
	// this differs from storj.ListOptions by removing the Delimiter field
	// (ours is hardcoded as "/"), and adding the Fields field to optionally
	// support efficient listing that doesn't require looking outside of the
	// path index in pointerdb.

	Prefix    storj.Path
	Cursor    storj.Path
	Recursive bool
	Direction storj.ListDirection
	Limit     int
	Fields    ListObjectsFields
}

// ListObjectsFields is an interface that I haven't figured out yet
type ListObjectsFields interface{}

// Range returns an objects data
func (o *Object) Range(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	return nil, nil
}
