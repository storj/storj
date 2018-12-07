package discovery

import (
	"context"

	"storj.io/storj/pkg/overlay"
)

// Refresh updates the cache db with the current DHT.
// We currently do not penalize nodes that are unresponsive,
// but should in the future.
func Refresh(ctx context.Context) error {
	// TODO(coyle): make refresh work by looking on the network for new ndoes
	o := overlay.LoadFromContext(ctx)
	nodes := o.DHT.Seen()

	for _, v := range nodes {
		if err := o.Put(ctx, v.Id, *v); err != nil {
			return err
		}
	}

	return nil
}

// Bootstrap walks the initialized network and populates the cache
func Bootstrap(ctx context.Context) error {
	// o := overlay.LoadFromContext(ctx)
	// kad := kademlia.LoadFromContext(ctx)
	// TODO(coyle): make Bootstrap work
	// look in our routing table
	// get every node we know about
	// ask every node for every node they know about
	// for each newly known node, ask those nodes for every node they know about
	// continue until no new nodes are found
	return nil
}
