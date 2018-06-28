package pool

import (
	"context"

	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

// Pool is a set of actions for maintaining a node connection pool
type Pool interface {
	Add(ctx context.Context, node node.Client) error
	Get(ctx context.Context, node proto.Node) error
	Remove(ctx context.Context, node proto.Node) error
}

// NewPool initializes a new connection pool
func NewPool() Pool {
	return &MemPool{}
}
