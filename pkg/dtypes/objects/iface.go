// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/dtypes"
	"storj.io/storj/pkg/ranger"
)

type ObjectStore interface {
	PutObject(ctx context.Context, path dtypes.Path, data io.Reader,
		metadata []byte, expiration time.Time) error
	GetObject(ctx context.Context, path dtypes.Path) (ranger.Ranger, dtypes.Meta,
		error)
	DeleteObject(ctx context.Context, path dtypes.Path) error
	ListObjects(ctx context.Context, startingPath, endingPath dtypes.Path) (
		paths []dtypes.Path, truncated bool, err error)

	SetXAttr(ctx context.Context, path dtypes.Path, xattr string,
		data io.Reader, metadata []byte) error
	GetXAttr(ctx context.Context, path dtypes.Path, xattr string) (ranger.Ranger,
		dtypes.Meta, error)
	DeleteXAttr(ctx context.Context, path dtypes.Path, xattr string) error
	ListXAttrs(ctx context.Context, path dtypes.Path,
		startingXAttr, endingXAttr string) (xattrs []string, truncated bool,
		err error)
}
