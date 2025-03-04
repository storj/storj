// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"io"
	"time"

	"storj.io/storj/cmd/uplink/ulloc"
	"storj.io/uplink"
)

// CreateOptions contains extra options to create an object.
type CreateOptions struct {
	Expires    time.Time
	Metadata   map[string]string
	SinglePart bool
}

// ListOptions describes options to the List command.
type ListOptions struct {
	Recursive   bool
	Pending     bool
	Expanded    bool
	AllVersions bool
}

func (lo *ListOptions) isRecursive() bool { return lo != nil && lo.Recursive }
func (lo *ListOptions) isPending() bool   { return lo != nil && lo.Pending }
func (lo *ListOptions) isExpanded() bool  { return lo != nil && lo.Expanded }
func (lo *ListOptions) allVersions() bool { return lo != nil && lo.AllVersions }

// RemoveOptions describes options to the Remove command.
type RemoveOptions struct {
	Version                   []byte
	BypassGovernanceRetention bool
	Pending                   bool
}

func (ro *RemoveOptions) isPending() bool { return ro != nil && ro.Pending }

// Filesystem represents either the local filesystem or the data backed by a project.
type Filesystem interface {
	Close() error
	Open(ctx context.Context, loc ulloc.Location) (MultiReadHandle, error)
	Create(ctx context.Context, loc ulloc.Location, opts *CreateOptions) (MultiWriteHandle, error)
	Move(ctx context.Context, source, dest ulloc.Location) error
	Copy(ctx context.Context, source, dest ulloc.Location) error
	Remove(ctx context.Context, loc ulloc.Location, opts *RemoveOptions) error
	List(ctx context.Context, prefix ulloc.Location, opts *ListOptions) (ObjectIterator, error)
	IsLocalDir(ctx context.Context, loc ulloc.Location) bool
	Stat(ctx context.Context, loc ulloc.Location) (*ObjectInfo, error)
}

// FilesystemLocal is the interface for a local filesystem.
type FilesystemLocal interface {
	IsLocalDir(ctx context.Context, path string) bool
	Open(ctx context.Context, path string) (MultiReadHandle, error)
	Create(ctx context.Context, path string) (MultiWriteHandle, error)
	Move(ctx context.Context, oldpath string, newpath string) error
	Copy(ctx context.Context, oldpath string, newpath string) error
	Remove(ctx context.Context, path string, opts *RemoveOptions) error
	List(ctx context.Context, path string, opts *ListOptions) (ObjectIterator, error)
	Stat(ctx context.Context, path string) (*ObjectInfo, error)
}

// FilesystemRemote is the interface for a remote filesystem.
type FilesystemRemote interface {
	Close() error
	Open(ctx context.Context, bucket, key string) (MultiReadHandle, error)
	Create(ctx context.Context, bucket, key string, opts *CreateOptions) (MultiWriteHandle, error)
	Move(ctx context.Context, oldbucket, oldkey string, newbucket, newkey string) error
	Copy(ctx context.Context, oldbucket, oldkey string, newbucket, newkey string) error
	Remove(ctx context.Context, bucket, key string, opts *RemoveOptions) error
	List(ctx context.Context, bucket, key string, opts *ListOptions) ObjectIterator
	Stat(ctx context.Context, bucket, key string) (*ObjectInfo, error)
}

//
// object info
//

// ObjectInfo is a simpler *uplink.Object that contains the minimal information the
// uplink command needs that multiple types can be converted to.
type ObjectInfo struct {
	Loc            ulloc.Location
	Version        []byte
	IsPrefix       bool
	IsDeleteMarker bool
	Created        time.Time
	ContentLength  int64
	Expires        time.Time
	Metadata       uplink.CustomMetadata
}

// uplinkObjectToObjectInfo returns an objectInfo converted from an *uplink.Object.
func uplinkObjectToObjectInfo(bucket string, obj *uplink.Object) ObjectInfo {
	return ObjectInfo{
		Loc:           ulloc.NewRemote(bucket, obj.Key),
		IsPrefix:      obj.IsPrefix,
		Created:       obj.System.Created,
		ContentLength: obj.System.ContentLength,
		Expires:       obj.System.Expires,
		Metadata:      obj.Custom,
	}
}

// uplinkUploadInfoToObjectInfo returns an objectInfo converted from an *uplink.Object.
func uplinkUploadInfoToObjectInfo(bucket string, upl *uplink.UploadInfo) ObjectInfo {
	return ObjectInfo{
		Loc:           ulloc.NewRemote(bucket, upl.Key),
		IsPrefix:      upl.IsPrefix,
		Created:       upl.System.Created,
		ContentLength: upl.System.ContentLength,
		Expires:       upl.System.Expires,
		Metadata:      upl.Custom,
	}
}

//
// read handles
//

// MultiReadHandle allows one to read different sections of something.
// The offset parameter can be negative to signal that the offset should
// start that many bytes back from the end. Any negative value for length
// indicates to read up to the end.
//
// TODO: A negative offset requires a negative length, but there is no
// reason why that must be so.
type MultiReadHandle interface {
	io.Closer
	SetOffset(offset int64) error
	NextPart(ctx context.Context, length int64) (ReadHandle, error)
	Info(ctx context.Context) (*ObjectInfo, error)
	Length() int64
}

// ReadHandle is something that can be read from distinct parts possibly
// in parallel.
type ReadHandle interface {
	io.Closer
	io.Reader
	Info() ObjectInfo
}

//
// write handles
//

// MultiWriteHandle lets one create multiple sequential WriteHandles for
// different sections of something.
//
// The returned WriteHandle will error if data is attempted to be written
// past the provided length. A negative length implies an unknown amount
// of data, and future calls to NextPart will error.
type MultiWriteHandle interface {
	NextPart(ctx context.Context, length int64) (WriteHandle, error)
	Commit(ctx context.Context) error
	Abort(ctx context.Context) error
}

// WriteHandle is anything that can be written to with commit/abort semantics.
type WriteHandle interface {
	io.Writer
	Commit() error
	Abort() error
}

//
// object iteration
//

// ObjectIterator is an interface type for iterating over objectInfo values.
type ObjectIterator interface {
	Next() bool
	Err() error
	Item() ObjectInfo
}
