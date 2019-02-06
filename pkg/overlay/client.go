// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
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

// client is the overlay concrete implementation of the client interface
type client struct {
	conn pb.OverlayClient
}

// Options contains parameters for selecting nodes
type Options struct {
	Amount       int
	Space        int64
	Bandwidth    int64
	Uptime       float64
	UptimeCount  int64
	AuditSuccess float64
	AuditCount   int64
	Excluded     storj.NodeIDList
}

// NewClient returns a new intialized Overlay Client
func NewClient(identity *identity.FullIdentity, address string) (Client, error) {
	return NewClientContext(context.TODO(), identity, address)
}

// NewClientContext returns a new intialized Overlay Client
func NewClientContext(ctx context.Context, identity *identity.FullIdentity, address string) (Client, error) {
	tc := transport.NewClient(identity, &Cache{}) // add overlay to transport client as observer
	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return nil, err
	}

	return &client{
		conn: pb.NewOverlayClient(conn),
	}, nil
}

// NewClientFrom returns a new overlay.Client from a connection
func NewClientFrom(conn pb.OverlayClient) Client { return &client{conn} }

// a compiler trick to make sure *client implements Client
var _ Client = (*client)(nil)

// Choose returns nodes based on Options
func (client *client) Choose(ctx context.Context, op Options) ([]*pb.Node, error) {
	var exIDs storj.NodeIDList
	exIDs = append(exIDs, op.Excluded...)
	// TODO(coyle): We will also need to communicate with the reputation service here
	resp, err := client.conn.FindStorageNodes(ctx, &pb.FindStorageNodesRequest{
		Opts: &pb.OverlayOptions{
			Amount:        int64(op.Amount),
			Restrictions:  &pb.NodeRestrictions{FreeDisk: op.Space, FreeBandwidth: op.Bandwidth},
			ExcludedNodes: exIDs,
		},
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return resp.GetNodes(), nil
}

// Lookup provides a Node with the given ID
func (client *client) Lookup(ctx context.Context, nodeID storj.NodeID) (*pb.Node, error) {
	resp, err := client.conn.Lookup(ctx, &pb.LookupRequest{NodeId: nodeID})
	if err != nil {
		return nil, err
	}

	return resp.GetNode(), nil
}

// BulkLookup provides a list of Nodes with the given IDs
func (client *client) BulkLookup(ctx context.Context, nodeIDs storj.NodeIDList) ([]*pb.Node, error) {
	var reqs pb.LookupRequests
	for _, v := range nodeIDs {
		reqs.LookupRequest = append(reqs.LookupRequest, &pb.LookupRequest{NodeId: v})
	}
	resp, err := client.conn.BulkLookup(ctx, &reqs)

	if err != nil {
		return nil, ClientError.Wrap(err)
	}

	var nodes []*pb.Node
	for _, v := range resp.LookupResponse {
		nodes = append(nodes, v.Node)
	}
	return nodes, nil
}
