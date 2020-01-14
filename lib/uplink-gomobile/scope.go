// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"context"

	"storj.io/common/fpath"
)

type scope struct {
	ctx    context.Context
	cancel func()
}

func rootScope(tempDir string) scope {
	ctx := context.Background()
	if tempDir == "inmemory" {
		ctx = fpath.WithTempData(ctx, "", true)
	} else {
		ctx = fpath.WithTempData(ctx, tempDir, false)
	}
	ctx, cancel := context.WithCancel(ctx)
	return scope{ctx, cancel}
}

func (parent *scope) child() scope {
	ctx, cancel := context.WithCancel(parent.ctx)
	return scope{ctx, cancel}
}
