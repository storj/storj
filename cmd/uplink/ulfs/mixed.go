// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulloc"
)

// Mixed dispatches to either the local or remote filesystem depending on the location.
type Mixed struct {
	local  FilesystemLocal
	remote FilesystemRemote
}

// NewMixed returns a Mixed backed by the provided local and remote filesystems.
func NewMixed(local FilesystemLocal, remote FilesystemRemote) *Mixed {
	return &Mixed{
		local:  local,
		remote: remote,
	}
}

// Close releases any resources that the Mixed contails.
func (m *Mixed) Close() error {
	return m.remote.Close()
}

// Open returns a MultiReadHandle to either a local file, remote object, or stdin.
func (m *Mixed) Open(ctx context.Context, loc ulloc.Location) (MultiReadHandle, error) {
	if bucket, key, ok := loc.RemoteParts(); ok {
		return m.remote.Open(ctx, bucket, key)
	} else if path, ok := loc.LocalParts(); ok {
		return m.local.Open(ctx, path)
	}
	return newStdMultiReadHandle(clingy.Stdin(ctx)), nil
}

// Create returns a WriteHandle to either a local file, remote object, or stdout.
func (m *Mixed) Create(ctx context.Context, loc ulloc.Location, opts *CreateOptions) (MultiWriteHandle, error) {
	if bucket, key, ok := loc.RemoteParts(); ok {
		return m.remote.Create(ctx, bucket, key, opts)
	} else if path, ok := loc.LocalParts(); ok {
		return m.local.Create(ctx, path)
	}
	return newStdMultiWriteHandle(clingy.Stdout(ctx)), nil
}

// Move moves either a local file or remote object.
func (m *Mixed) Move(ctx context.Context, source, dest ulloc.Location) error {
	if oldbucket, oldkey, ok := source.RemoteParts(); ok {
		if newbucket, newkey, ok := dest.RemoteParts(); ok {
			return m.remote.Move(ctx, oldbucket, oldkey, newbucket, newkey)
		}
	} else if oldpath, ok := source.LocalParts(); ok {
		if newpath, ok := dest.LocalParts(); ok {
			return m.local.Move(ctx, oldpath, newpath)
		}
	}
	return errs.New("moving objects between local and remote is not supported")
}

// Copy copies either a local file or remote object.
func (m *Mixed) Copy(ctx context.Context, source, dest ulloc.Location) error {
	if oldbucket, oldkey, ok := source.RemoteParts(); ok {
		if newbucket, newkey, ok := dest.RemoteParts(); ok {
			return m.remote.Copy(ctx, oldbucket, oldkey, newbucket, newkey)
		}
	} else if oldpath, ok := source.LocalParts(); ok {
		if newpath, ok := dest.LocalParts(); ok {
			return m.local.Copy(ctx, oldpath, newpath)
		}
	}
	return errs.New("copying objects between local and remote is not supported")
}

// Remove deletes either a local file or remote object.
func (m *Mixed) Remove(ctx context.Context, loc ulloc.Location, opts *RemoveOptions) error {
	if bucket, key, ok := loc.RemoteParts(); ok {
		return m.remote.Remove(ctx, bucket, key, opts)
	} else if path, ok := loc.LocalParts(); ok {
		return m.local.Remove(ctx, path, opts)
	}
	return nil
}

// List lists either files and directories with some local path prefix or remote objects
// with a given bucket and key.
func (m *Mixed) List(ctx context.Context, prefix ulloc.Location, opts *ListOptions) (ObjectIterator, error) {
	if bucket, key, ok := prefix.RemoteParts(); ok {
		return m.remote.List(ctx, bucket, key, opts), nil
	} else if path, ok := prefix.LocalParts(); ok {
		return m.local.List(ctx, path, opts)
	}
	return nil, errs.New("unable to list objects for prefix %q", prefix)
}

// IsLocalDir returns true if the location is a directory that is local.
func (m *Mixed) IsLocalDir(ctx context.Context, loc ulloc.Location) bool {
	if path, ok := loc.LocalParts(); ok {
		return m.local.IsLocalDir(ctx, path)
	}
	return false
}

// Stat returns information about an object at the specified Location.
func (m *Mixed) Stat(ctx context.Context, loc ulloc.Location) (*ObjectInfo, error) {
	if bucket, key, ok := loc.RemoteParts(); ok {
		return m.remote.Stat(ctx, bucket, key)
	} else if path, ok := loc.LocalParts(); ok {
		return m.local.Stat(ctx, path)
	}
	return nil, errs.New("unable to stat loc %q", loc.Loc())
}
