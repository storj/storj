// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package connectioncache

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/clients/transportclient"
	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

//NewConnCache returns ConnCache interface
func NewConnectionCache(ctx context.Context, addr string, tc Client) (ConnectionCache, error) {
	switch tc {
	case Overlay:
		c, err := overlay.NewOverlayClient(addr)
		if err != nil {
			return nil, err
		}
		return &connectionCache{
			overlayClient: c,
		}, err
	case NetState:
		return nil, errors.New("Need to talk to Nat, to implement similar to Overlay")
	case PieceStore:
		return nil, errors.New("need to talk to Alex, to implement similar to Overlay")
	default:
		return nil, errors.New("Unsupported-TransportClientType")
	}
}

// Dial using the authenticated mode
func (o *connectionCache) DialNode(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)
	if node == nil {
		return nil, errors.New("node param uninitialized")
	} else {
		if node.Address == nil {
			/* check to see nodeID is present to look up for the corresponding address */
			if node.GetId() != "" {
				lookupNode, err := o.overlayClient.Lookup(ctx, overlay.NodeID(node.GetId()))
				if err != nil {
					return nil, err
				}
				/* err is nil, that means lookup passed complete info */
				conn, err = transportclient.DialNode(ctx, lookupNode.Address.Address)
				if err != nil {
					return nil, err
				}
				node.Address.Address = lookupNode.Address.Address
			} else {
				return nil, errors.New("node Address uninitialized")
			}
		} else {
			conn, err = transportclient.DialNode(ctx, node.Address.Address)
			if err != nil {
				return nil, err
			}
		}
		return conn, err
	}
}

// Dial using unauthenticated mode
func (o *connectionCache) DialUnauthenticated(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)
	if node == nil {
		return nil, errors.New("node param uninitialized")
	} else {
		if node.Address == nil {
			/* check to see nodeID is present to look up for the corresponding address */
			if node.GetId() != "" {
				lookupNode, err := o.overlayClient.Lookup(ctx, overlay.NodeID(node.GetId()))
				if err != nil {
					return nil, err
				}
				/* err is nil, that means lookup passed complete info */
				conn, err = transportclient.DialUnauthenticated(ctx, lookupNode.Address.Address)
				if err != nil {
					return nil, err
				}
				/* TODO@ASK add cacheing mechanism */
				node.Address.Address = lookupNode.Address.Address
			} else {
				return nil, errors.New("node Address uninitialized")
			}
		} else {
			conn, err = transportclient.DialUnauthenticated(ctx, node.Address.Address)
			if err != nil {
				return nil, err
			}
		}
		return conn, err
	}
}

// // GetTransportClientType returns the transportclient.Type of ClientConn.
// // This is an EXPERIMENTAL API.
// func GetClient(tc transportclient.Client) string {
// 	return tc.String()
// }
