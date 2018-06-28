package pool

import (
	"context"

	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

// MemPool is the in memor implementation of a connection Pool
type MemPool struct {
}

// Add (TODO)
func (mp *MemPool) Add(ctx context.Context, node node.Client) error {
	return nil
}

// Get (TODO)
func (mp *MemPool) Get(ctx context.Context, node proto.Node) error {
	return nil
}

// Remove (TODO)
func (mp *MemPool) Remove(ctx context.Context, node proto.Node) error {
	return nil
}
