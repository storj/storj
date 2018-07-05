// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
)

var (
	mon = monkit.Package()
	//Error is the errs class of standard Object Store errors
	Error = errs.Class("objectstore error")
)

//ObjectStore interface
type ObjectStore interface {
	PutObject(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) error
	GetObject(ctx context.Context, path paths.Path) (ranger.RangeCloser, Meta, error)
	DeleteObject(ctx context.Context, path paths.Path) error
	ListObjects(ctx context.Context, startingPath, endingPath paths.Path) (paths []paths.Path, truncated bool, err error)
	SetXAttr(ctx context.Context, path paths.Path, xattr string, data io.Reader, metadata []byte) error
	GetXAttr(ctx context.Context, path paths.Path, xattr string) (ranger.RangeCloser, Meta, error)
	DeleteXAttr(ctx context.Context, path paths.Path, xattr string) error
	ListXAttrs(ctx context.Context, path paths.Path, startingXAttr, endingXAttr string) (xattrs []string, truncated bool, err error)
}
