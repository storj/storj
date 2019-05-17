// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"context"

	"storj.io/storj/internal/fpath"
)

type scope struct {
	ctx    context.Context
	cancel func()
}

func rootScope(tempDir string) scope {
	ctx := fpath.WithTempDir(context.Background(), tempDir)
	ctx, cancel := context.WithCancel(ctx)
	return scope{ctx, cancel}
}

func (parent *scope) child() scope {
	ctx, cancel := context.WithCancel(parent.ctx)
	return scope{ctx, cancel}
}
