package mobile

import (
	"context"
)

type scope struct {
	ctx    context.Context
	cancel func()
}

func rootScope(tmpDir string) scope {
	ctx := context.Background()
	// TODO make type for this
	if tmpDir == "" {
		ctx = context.WithValue(ctx, "writableDir", "inmemory")
	} else {
		ctx = context.WithValue(ctx, "writableDir", tmpDir)
	}

	ctx, cancel := context.WithCancel(ctx)
	return scope{ctx, cancel}
}

func (parent *scope) child() scope {
	ctx, cancel := context.WithCancel(parent.ctx)
	return scope{ctx, cancel}
}
