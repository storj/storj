// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
)

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

// GetObject returns a handle to the data for an object and its metadata, if
// authorized.
func (s *Session) GetObject(ctx context.Context, bucket string, path storj.Path) (
	ranger.Ranger, ObjectMeta, error) {

	return nil, ObjectMeta{}, nil
}

// ObjectPutOpts controls options about uploading a new Object, if authorized.
type ObjectPutOpts struct {
	Metadata map[string]string
	Expires  time.Time

	// the satellite should probably tell the uplink what to use for these
	// per bucket. also these should probably be denormalized and defined here.
	RS            *storj.RedundancyScheme
	NodeSelection *miniogw.NodeSelectionConfig
}

// Upload uploads a new object, if authorized.
func (s *Session) Upload(ctx context.Context, bucket string, path storj.Path,
	data io.Reader, opts ObjectPutOpts) error {
	panic("TODO")
}

// DeleteObject removes an object, if authorized.
func (s *Session) DeleteObject(ctx context.Context, bucket string,
	path storj.Path) error {
	panic("TODO")
}

// ListObjectsField numbers the fields of list objects
type ListObjectsField int

const (
	ListObjectsMetaNone        ListObjectsField = 0
	ListObjectsMetaModified    ListObjectsField = 1 << iota
	ListObjectsMetaExpiration  ListObjectsField = 1 << iota
	ListObjectsMetaSize        ListObjectsField = 1 << iota
	ListObjectsMetaChecksum    ListObjectsField = 1 << iota
	ListObjectsMetaUserDefined ListObjectsField = 1 << iota
	ListObjectsMetaAll         ListObjectsField = 1 << iota
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

// ListObjects lists objects a user is authorized to see.
func (s *Session) ListObjects(ctx context.Context, bucket string,
	cfg ListObjectsConfig) (items []ObjectMeta, more bool, err error) {

	// TODO: wire up ListObjectsV2

	// s.Gateway.ListObjectsV2(bucket, cfg.Prefix, "/", cfg.Limit)
	panic("TODO")
}
