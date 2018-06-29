// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/ranger"
)

var (
	mon = monkit.Package()
	//Error monkit
	Error = errs.Class("error")
)

//ObjectStore interface
type ObjectStore interface {
	PutObject(ctx context.Context, path string, data io.Reader, metadata []byte, expiration time.Time) error
	GetObject(ctx context.Context, path string) (ranger.Ranger, Meta, error)
	DeleteObject(ctx context.Context, path string) error
	ListObjects(ctx context.Context, startingPath, endingPath string) (paths []string, truncated bool, err error)
	SetXAttr(ctx context.Context, path, xattr string, data io.Reader, metadata []byte) error
	GetXAttr(ctx context.Context, path, xattr string) (ranger.Ranger, Meta, error)
	DeleteXAttr(ctx context.Context, path, xattr string) error
	ListXAttrs(ctx context.Context, path, startingXAttr, endingXAttr string) (xattrs []string, truncated bool, err error)
}
