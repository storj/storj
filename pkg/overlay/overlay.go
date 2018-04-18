package overlay

import (
	"context"

	"github.com/coyle/storj/protos/overlay"
)

// Overlay implements our overlay RPC service
type Overlay struct{}

// Lookup finds the address of a node in our overlay network
func (o *Overlay) Lookup(ctx context.Context, req *overlay.LookupRequest) (*overlay.LookupResponse, error) {
	// TODO: fill this in with logic to communicate with kademlia
	return nil, nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Overlay) FindStorageNodes(ctx context.Context, req *overlay.FindStorageNodesRequest) (*overlay.FindStorageNodesResponse, error) {
	// TODO: fill this in with logic to communicate with kademlia
	return nil, nil
}
