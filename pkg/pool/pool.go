// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package pool

import (
	"context"
)

// Pool is a set of actions for maintaining a node connection pool
type Pool interface {
	Add(ctx context.Context, key string, value interface{}) error
	Get(ctx context.Context, key string) (interface{}, error)
	Remove(ctx context.Context, key string) error
	Disconnect(ctx context.Context) error
	Init()
}
