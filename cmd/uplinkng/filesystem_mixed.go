// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/clingy"
)

//
// filesystemMixed dispatches to either the local or remote filesystem depending on the path
//

type filesystemMixed struct {
	local  *filesystemLocal
	remote *filesystemRemote
}

func (m *filesystemMixed) Close() error {
	return m.remote.Close()
}

func (m *filesystemMixed) Open(ctx clingy.Context, loc Location) (readHandle, error) {
	if loc.Remote() {
		return m.remote.Open(ctx, loc.bucket, loc.key)
	} else if loc.Std() {
		return newGenericReadHandle(ctx.Stdin()), nil
	}
	return m.local.Open(ctx, loc.path)
}

func (m *filesystemMixed) Create(ctx clingy.Context, loc Location) (writeHandle, error) {
	if loc.Remote() {
		return m.remote.Create(ctx, loc.bucket, loc.key)
	} else if loc.Std() {
		return newGenericWriteHandle(ctx.Stdout()), nil
	}
	return m.local.Create(ctx, loc.path)
}

func (m *filesystemMixed) ListObjects(ctx context.Context, prefix Location, recursive bool) (objectIterator, error) {
	if prefix.Remote() {
		return m.remote.ListObjects(ctx, prefix.bucket, prefix.key, recursive), nil
	}
	return m.local.ListObjects(ctx, prefix.path, recursive)
}

func (m *filesystemMixed) ListUploads(ctx context.Context, prefix Location, recursive bool) (objectIterator, error) {
	if prefix.Remote() {
		return m.remote.ListPendingMultiparts(ctx, prefix.bucket, prefix.key, recursive), nil
	}
	return emptyObjectIterator{}, nil
}

func (m *filesystemMixed) IsLocalDir(ctx context.Context, loc Location) bool {
	if !loc.Local() {
		return false
	}
	return m.local.IsLocalDir(ctx, loc.path)
}
