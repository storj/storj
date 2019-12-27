// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"storj.io/common/fpath"
)

// scope implements nesting context for foreign api.
type scope struct {
	ctx    context.Context
	cancel func()
}

// rootScope creates a scope with the specified temp directory.
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

// child creates an inherited scope.
func (parent *scope) child() scope {
	ctx, cancel := context.WithCancel(parent.ctx)
	return scope{ctx, cancel}
}
