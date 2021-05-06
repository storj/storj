// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulloc"
)

// Mixed dispatches to either the local or remote filesystem depending on the location.
type Mixed struct {
	local  *Local
	remote *Remote
}

// NewMixed returns a Mixed backed by the provided local and remote filesystems.
func NewMixed(local *Local, remote *Remote) *Mixed {
	return &Mixed{
		local:  local,
		remote: remote,
	}
}

// Close releases any resources that the Mixed contails.
func (m *Mixed) Close() error {
	return m.remote.Close()
}

// Open returns a ReadHandle to either a local file, remote object, or stdin.
func (m *Mixed) Open(ctx clingy.Context, loc ulloc.Location) (ReadHandle, error) {
	if bucket, key, ok := loc.RemoteParts(); ok {
		return m.remote.Open(ctx, bucket, key)
	} else if path, ok := loc.LocalParts(); ok {
		return m.local.Open(ctx, path)
	}
	return newGenericReadHandle(ctx.Stdin()), nil
}

// Create returns a WriteHandle to either a local file, remote object, or stdout.
func (m *Mixed) Create(ctx clingy.Context, loc ulloc.Location) (WriteHandle, error) {
	if bucket, key, ok := loc.RemoteParts(); ok {
		return m.remote.Create(ctx, bucket, key)
	} else if path, ok := loc.LocalParts(); ok {
		return m.local.Create(ctx, path)
	}
	return newGenericWriteHandle(ctx.Stdout()), nil
}

// ListObjects lists either files and directories with some local path prefix or remote objects
// with a given bucket and key.
func (m *Mixed) ListObjects(ctx context.Context, prefix ulloc.Location, recursive bool) (ObjectIterator, error) {
	if bucket, key, ok := prefix.RemoteParts(); ok {
		return m.remote.ListObjects(ctx, bucket, key, recursive), nil
	} else if path, ok := prefix.LocalParts(); ok {
		return m.local.ListObjects(ctx, path, recursive)
	}
	return nil, errs.New("unable to list objects for prefix %q", prefix)
}

// ListUploads lists all of the pending uploads for remote objects with some given bucket and key.
func (m *Mixed) ListUploads(ctx context.Context, prefix ulloc.Location, recursive bool) (ObjectIterator, error) {
	if bucket, key, ok := prefix.RemoteParts(); ok {
		return m.remote.ListUploads(ctx, bucket, key, recursive), nil
	} else if prefix.Local() {
		return emptyObjectIterator{}, nil
	}
	return nil, errs.New("unable to list uploads for prefix %q", prefix)
}

// IsLocalDir returns true if the location is a directory that is local.
func (m *Mixed) IsLocalDir(ctx context.Context, loc ulloc.Location) bool {
	if path, ok := loc.LocalParts(); ok {
		return m.local.IsLocalDir(ctx, path)
	}
	return false
}
