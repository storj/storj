// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package pool

import (
	"context"

	"github.com/zeebo/errs"
)

// PoolError is the class of errors for the pool package
var PoolError = errs.Class("pool error")

// Pool is a set of actions for maintaining a node connection pool
type Pool interface {
	Add(ctx context.Context, key string, value interface{}) error
	Get(ctx context.Context, key string) (interface{}, error)
	Remove(ctx context.Context, key string) error
}
