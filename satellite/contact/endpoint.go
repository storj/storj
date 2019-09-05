// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
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
	pingNodeSuccess, pingErrorMessage, err := pingBack(ctx, endpoint, req, peerID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Save this uptime update checkin in a cache so we can batch write
	// to overlay database later
	update := overlay.NodeCheckinInfo{
		NodeID:         peerID,
		IsUp:           pingNodeSuccess,
		OperatorWallet: req.GetOperator().GetWallet(),
		OperatorEmail:  req.GetOperator().GetEmail(),
		FreeDisk:       req.GetCapacity().GetFreeDisk(),
		FreeBandwidth:  req.GetCapacity().GetFreeBandwidth(),
	}
	err = endpoint.service.AddUpdateToCache(ctx, &update)
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
		return false, "", Error.New("couldn't connect to client at addr: %s. Error: %v.", req.GetAddress().String(), err)
	}

	pingNodeSuccess := true
	var pingErrorMessage string

	p := &peer.Peer{}
	_, err = client.pingNode(ctx, &pb.ContactPingRequest{}, grpc.Peer(p))
	if err != nil {
		pingNodeSuccess = false
		// TODO: check common errors codes
		pingErrorMessage = fmt.Sprintf("erroring while trying to pingNode: %v\n", err)
	}
	identityFromPeer, err := identity.PeerIdentityFromPeer(p)
	if err != nil {
		return false, "", Error.New("couldn't get identity from peer:", err)
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
