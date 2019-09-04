// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
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

	pingNodeSuccess, pingErrorMessage, err := pingBack(ctx, endpoint, req)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Update the overlay cache with new uptime info from pinging back the node
	peerID, err := peerIDFromContext(ctx)
	_, err = endpoint.service.overlay.UpdateUptime(ctx, peerID, pingNodeSuccess)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Should we update the req.NodeCapacity here as well?

	return &pb.CheckinResponse{
		PingNodeSuccess:  pingNodeSuccess,
		PingErrorMessage: pingErrorMessage,
	}, nil
}

func pingBack(ctx context.Context, endpoint *Endpoint, req *pb.CheckinRequest) (bool, string, error) {
	client, err := newClient(ctx,
		endpoint.log,
		endpoint.service.transport,
		req.GetAddress(),
	)
	defer func() {
		err = errs.Combine(err, client.close())
	}()
	if err != nil {
		return false, "", err
	}

	pingNodeSuccess := true
	var pingErrorMessage string

	_, err = client.pingNode(ctx, &pb.ContactPingRequest{})
	if err != nil {
		pingNodeSuccess = false
		pingErrorMessage = err.Error()
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

	// TODO: call DB.PeerIdentities().Set(context.Context, storj.NodeID, *identity.PeerIdentity)
	// to verify a node's latest peer identity signed
	return peerID.ID, nil
}
