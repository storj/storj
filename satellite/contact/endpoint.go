// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Endpoint implements the contact service Endpoints.
type Endpoint struct {
	log     *zap.Logger
	service *Service
}

// NewEndpoint returns a new contact service endpoint.
func NewEndpoint(log *zap.Logger, service *Service) *Endpoint {
	return &Endpoint{
		log:     log,
		service: service,
	}
}

// Checkin is periodically called by storage nodes to keep the satellite informed of its existence,
// address, and operator information. In return, this satellite keeps the node informed of its
// reachability.
// When a node checkins with the satellite, the satellite pings the node back to confirm they can
// successfully connect.
func (endpoint *Endpoint) Checkin(ctx context.Context, req *pb.CheckinRequest) (_ *pb.CheckinResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peerID, err := peerIDFromContext(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	pingNodeSuccess, pingErrorMessage, err := pingBack(ctx, endpoint, req, peerID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// TODO(jg): We are making 2 requests to the database, one to update uptime and
	// the other to update the capacity and operator info. We should combine these into
	// one to reduce db connections. Consider adding batching and using a stored procedure.
	_, err = endpoint.service.overlay.UpdateUptime(ctx, peerID, pingNodeSuccess)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	nodeInfo := pb.InfoResponse{Operator: req.GetOperator(), Capacity: req.GetCapacity()}
	_, err = endpoint.service.overlay.UpdateNodeInfo(ctx, peerID, &nodeInfo)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &pb.CheckinResponse{
		PingNodeSuccess:  pingNodeSuccess,
		PingErrorMessage: pingErrorMessage,
	}, nil
}

func pingBack(ctx context.Context, endpoint *Endpoint, req *pb.CheckinRequest, peerIDFromContext storj.NodeID) (bool, string, error) {
	client, err := newClient(ctx,
		endpoint.log,
		endpoint.service.transport,
		req.GetAddress(),
	)
	if err != nil {
		// if this is a network error, then return the error otherwise just report internal error
		_, ok := err.(net.Error)
		if ok {
			return false, "", Error.New("failed to connect to %s: %v", req.GetAddress().String(), err)
		}
		endpoint.log.Info("pingBack internal error", zap.String("error", err.Error()))
		return false, "", Error.New("couldn't connect to client at addr: %s due to internal error.", req.GetAddress().String())
	}

	pingNodeSuccess := true
	var pingErrorMessage string

	p := &peer.Peer{}
	_, err = client.pingNode(ctx, &pb.ContactPingRequest{}, grpc.Peer(p))
	if err != nil {
		pingNodeSuccess = false
		pingErrorMessage = "erroring while trying to pingNode due to internal error"
		_, ok := err.(net.Error)
		if ok {
			pingErrorMessage = fmt.Sprintf("network erroring while trying to pingNode: %v\n", err)
		}
	}

	// Confirm that the node ID from the initial checkin request
	// matches that from this pingNode request
	identityFromPeer, err := identity.PeerIdentityFromPeer(p)
	if err != nil {
		return false, "", Error.New("couldn't get identity from peer: %v", err)
	}
	if identityFromPeer.ID != peerIDFromContext {
		return false, "", Error.New("peer ID from context, %s, does not match ID from ping request, %s.", peerIDFromContext.String(), identityFromPeer.ID.String())
	}

	return pingNodeSuccess, pingErrorMessage, nil
}

func peerIDFromContext(ctx context.Context) (storj.NodeID, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return storj.NodeID{}, Error.New("unable to get grpc peer from contex")
	}
	peerID, err := identity.PeerIdentityFromPeer(p)
	if err != nil {
		return storj.NodeID{}, err
	}
	return peerID.ID, nil
}
