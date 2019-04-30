package mobile

import (
	"context"
)

type scope struct {
	ctx    context.Context
	cancel func()
}

func rootScope() scope {
	ctx, cancel := context.WithCancel(context.Background())
	return scope{ctx, cancel}
}

func (parent *scope) child() scope {
	ctx, cancel := context.WithCancel(parent.ctx)
	return scope{ctx, cancel}
}
