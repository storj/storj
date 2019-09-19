// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"fmt"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

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

// CheckIn is periodically called by storage nodes to keep the satellite informed of its existence,
// address, and operator information. In return, this satellite keeps the node informed of its
// reachability.
// When a node checks-in with the satellite, the satellite pings the node back to confirm they can
// successfully connect.
func (endpoint *Endpoint) CheckIn(ctx context.Context, req *pb.CheckInRequest) (_ *pb.CheckInResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peerID, err := peerIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, Error.Wrap(err).Error())
	}
	nodeID := peerID.ID

	err = endpoint.service.peerIDs.Set(ctx, nodeID, peerID)
	if err != nil {
		return nil, status.Error(codes.Internal, Error.Wrap(err).Error())
	}

	pingNodeSuccess, pingErrorMessage, err := endpoint.pingBack(ctx, req, nodeID)
	if err != nil {
		return nil, status.Error(codes.Internal, Error.Wrap(err).Error())
	}

	err = endpoint.service.overlay.Put(ctx, nodeID, pb.Node{
		Id: nodeID,
		Address: &pb.NodeAddress{
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
			Address:   req.Address,
		},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, Error.Wrap(err).Error())
	}

	// TODO(jg): We are making 2 requests to the database, one to update uptime and
	// the other to update the capacity and operator info. We should combine these into
	// one to reduce db connections. Consider adding batching and using a stored procedure.
	_, err = endpoint.service.overlay.UpdateUptime(ctx, nodeID, pingNodeSuccess)
	if err != nil {
		return nil, status.Error(codes.Internal, Error.Wrap(err).Error())
	}

	nodeInfo := pb.InfoResponse{Operator: req.GetOperator(), Capacity: req.GetCapacity(), Version: &pb.NodeVersion{Version: req.Version}}
	_, err = endpoint.service.overlay.UpdateNodeInfo(ctx, nodeID, &nodeInfo)
	if err != nil {
		return nil, status.Error(codes.Internal, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("checking in", zap.String("node addr", req.Address), zap.Bool("ping node succes", pingNodeSuccess))
	return &pb.CheckInResponse{
		PingNodeSuccess:  pingNodeSuccess,
		PingErrorMessage: pingErrorMessage,
	}, nil
}

func (endpoint *Endpoint) pingBack(ctx context.Context, req *pb.CheckInRequest, peerID storj.NodeID) (bool, string, error) {
	client, err := newClient(ctx,
		endpoint.service.transport,
		req.Address,
		peerID,
	)
	if err != nil {
		// if this is a network error, then return the error otherwise just report internal error
		_, ok := err.(net.Error)
		if ok {
			return false, "", Error.New("failed to connect to %s: %v", req.Address, err)
		}
		endpoint.log.Info("pingBack internal error", zap.String("error", err.Error()))
		return false, "", Error.New("couldn't connect to client at addr: %s due to internal error.", req.Address)
	}
	defer func() {
		err = errs.Combine(err, client.close())
	}()

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

	return pingNodeSuccess, pingErrorMessage, err
}

func peerIDFromContext(ctx context.Context) (*identity.PeerIdentity, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, Error.New("unable to get grpc peer from context")
	}
	peerIdentity, err := identity.PeerIdentityFromPeer(p)
	if err != nil {
		return nil, err
	}
	return peerIdentity, nil
}
