// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package connectioncache

import (
	"context"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/overlay"
	proto "storj.io/storj/protos/overlay"
)

var (
	mon   = monkit.Package()
	Error = errs.Class("error")
)

// Client indicates the supported clients
// It can be any client type, with whom trying to establish connection.
type Client int

func (tc Client) String() string {
	switch tc {
	case Overlay:
		return "OVERLAYCLIENT"
	case NetState:
		return "NETSTATECLIENT"
	case PieceStore:
		return "PIECESTORE"
	default:
		return "Unsupported-Client"
	}
}

const (
	// Overlay indicates, try to establish connection session with NetworkState Client
	Overlay Client = iota
	// NetState indicates, try to establish connection session with NetworkState Client
	NetState
	// PieceStore indicates, try to establish connection session with PieceStore Client
	PieceStore
	// add here new clients
)

// ConnectionCache defines the interface to any network client.
type ConnectionCache interface {
	DialUnauthenticated(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)
	DialNode(ctx context.Context, node *proto.Node) (*grpc.ClientConn, error)

	/* TODO@ASK add here the cache supported connection and any other methods to encapsulate
	any of the inner working details about making connection or lookup or conn selection or
	cache mechanism/algorithm functionality */
}

// connectionCache is the concrete implementation for the clients to open up connection
type connectionCache struct {
	overlayClient *overlay.Overlay
}
