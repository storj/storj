// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// Client is the interface that defines an overlay client.
//
// Choose returns a list of storage NodeID's that fit the provided criteria.
// 	limit is the maximum number of nodes to be returned.
// 	space is the storage and bandwidth requested consumption in bytes.
//
// Lookup finds a Node with the provided identifier.

// ClientError creates class of errors for stack traces
var ClientError = errs.Class("Client Error")

//Client implements the Overlay Client interface
type Client interface {
	Choose(ctx context.Context, op Options) ([]*pb.Node, error)
	Lookup(ctx context.Context, nodeID storj.NodeID) (*pb.Node, error)
	BulkLookup(ctx context.Context, nodeIDs storj.NodeIDList) ([]*pb.Node, error)
}

// Overlay is the overlay concrete implementation of the client interface
type Overlay struct {
	client pb.OverlayClient
}

// Options contains parameters for selecting nodes
type Options struct {
	Amount   int
	Space    int64
	Excluded storj.NodeIDList
}

// NewOverlayClient returns a new intialized Overlay Client
func NewOverlayClient(identity *provider.FullIdentity, address string) (Client, error) {
	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(context.Background(), address)
	if err != nil {
		return nil, err
	}

	return &Overlay{
		client: pb.NewOverlayClient(conn),
	}, nil
}

// NewClientFrom returns a new overlay.Client from a connection
func NewClientFrom(conn pb.OverlayClient) Client { return &Overlay{conn} }

// a compiler trick to make sure *Overlay implements Client
var _ Client = (*Overlay)(nil)

// Choose implements the client.Choose interface
func (o *Overlay) Choose(ctx context.Context, op Options) ([]*pb.Node, error) {
	var exIDs storj.NodeIDList
	for _, id := range op.Excluded {
		exIDs = append(exIDs, id)
	}
	// TODO(coyle): We will also need to communicate with the reputation service here
	resp, err := o.client.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{
		Opts: &pb.OverlayOptions{
			Amount:        int64(op.Amount),
			Restrictions:  &pb.NodeRestrictions{FreeDisk: op.Space},
			ExcludedNodes: &exIDs,
		},
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return resp.GetNodes(), nil
}

// Lookup provides a Node with the given ID
func (o *Overlay) Lookup(ctx context.Context, nodeID storj.NodeID) (*pb.Node, error) {
	resp, err := o.client.Lookup(ctx, &pb.LookupRequest{NodeId: nodeID})
	if err != nil {
		return nil, err
	}

	return resp.GetNode(), nil
}

// BulkLookup provides a list of Nodes with the given IDs
func (o *Overlay) BulkLookup(ctx context.Context, nodeIDs storj.NodeIDList) ([]*pb.Node, error) {
	var reqs pb.LookupRequests
	for _, v := range nodeIDs {
		reqs.Lookuprequest = append(reqs.Lookuprequest, &pb.LookupRequest{NodeId: v})
	}
	resp, err := o.client.BulkLookup(ctx, &reqs)

	if err != nil {
		return nil, ClientError.Wrap(err)
	}

	var nodes []*pb.Node
	for _, v := range resp.Lookupresponse {
		nodes = append(nodes, v.Node)
	}
	return nodes, nil
}
