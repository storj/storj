package overlay

import (
	"context"

	"google.golang.org/grpc"
	proto "storj.io/storj/protos/overlay"
)

// Client defines the interface to an overlay client. The amount is the number of nodes you would like returned,
// space is the storage amount in kilobytes and bandwidth is the amount of bandwidth in kilobytes requested
//  over X amount of time
type Client interface {
	Choose(ctx context.Context, amount int, space, bw int) ([]NodeID, error)
	Lookup(ctx context.Context, nodeID NodeID) (*proto.Node, error)
}

// Overlay is the overlay concrete implementation of the client interface
type Overlay struct {
	client proto.OverlayClient
}

// NewOverlayClient returns a new intialized Overlay Client
func NewOverlayClient(address string) (*Overlay, error) {
	c, err := NewClient(&address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Overlay{
		client: c,
	}, nil
}

// Choose returns a list of storage NodeID's that fit the provided criteria
func (o *Overlay) Choose(ctx context.Context, amount int, space, bw int) ([]NodeID, error) {
	// TODO(coyle): We will also need to communicate with the reputation service here
	resp, err := o.client.FindStorageNodes(ctx, &proto.FindStorageNodesRequest{})
	if err != nil {
		return []NodeID{}, err
	}

	ids := []NodeID{}
	for _, v := range resp.GetNodes() {
		ids = append(ids, NodeID(v.GetId()))
	}
	return ids, nil
}

// Lookup provides a Node with the given address
func (o *Overlay) Lookup(ctx context.Context, nodeID NodeID) (*proto.Node, error) {
	resp, err := o.client.Lookup(ctx, &proto.LookupRequest{NodeID: nodeID.String()})
	if err != nil {
		return &proto.Node{}, err
	}

	return resp.GetNode(), nil
}
