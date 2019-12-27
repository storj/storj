// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"fmt"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

var (
	errPingBackDial    = errs.Class("pingback dialing error")
	errCheckInIdentity = errs.Class("check-in identity error")
	errCheckInNetwork  = errs.Class("check-in network error")
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

	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Info("failed to get node ID from context", zap.String("node address", req.Address), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Unknown, errCheckInIdentity.New("failed to get ID from context: %v", err).Error())
	}
	nodeID := peerID.ID

	err = endpoint.service.peerIDs.Set(ctx, nodeID, peerID)
	if err != nil {
		endpoint.log.Info("failed to add peer identity entry for ID", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, errCheckInIdentity.New("failed to add peer identity entry for ID: %v", err).Error())
	}

	lastIP, err := overlay.GetNetwork(ctx, req.Address)
	if err != nil {
		endpoint.log.Info("failed to resolve IP from address", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errCheckInNetwork.New("failed to resolve IP from address: %s, err: %v", req.Address, err).Error())
	}

	pingNodeSuccess, pingErrorMessage, err := endpoint.pingBack(ctx, req, nodeID)
	if err != nil {
		endpoint.log.Info("failed to ping back address", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		if errPingBackDial.Has(err) {
			err = errCheckInNetwork.New("failed dialing address when attempting to ping node (ID: %s): %s, err: %v", nodeID, req.Address, err)
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		}
		err = errCheckInNetwork.New("failed to ping node (ID: %s) at address: %s, err: %v", nodeID, req.Address, err)
		return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
	}
	nodeInfo := overlay.NodeCheckInInfo{
		NodeID: peerID.ID,
		Address: &pb.NodeAddress{
			Address:   req.Address,
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
		},
		LastIP:   lastIP,
		IsUp:     pingNodeSuccess,
		Capacity: req.Capacity,
		Operator: req.Operator,
		Version:  req.Version,
	}
	err = endpoint.service.overlay.UpdateCheckIn(ctx, nodeInfo, time.Now().UTC())
	if err != nil {
		endpoint.log.Info("failed to update check in", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("checking in", zap.String("node addr", req.Address), zap.Bool("ping node success", pingNodeSuccess), zap.String("ping node err msg", pingErrorMessage))
	return &pb.CheckInResponse{
		PingNodeSuccess:  pingNodeSuccess,
		PingErrorMessage: pingErrorMessage,
	}, nil
}

func (endpoint *Endpoint) pingBack(ctx context.Context, req *pb.CheckInRequest, peerID storj.NodeID) (_ bool, _ string, err error) {
	defer mon.Task()(&ctx)(&err)

	pingNodeSuccess := true
	var pingErrorMessage string

	client, err := newClient(ctx, endpoint.service.dialer, req.Address, peerID)
	if err != nil {
		// If there is an error from trying to dial and ping the node, return that error as
		// pingErrorMessage and not as the err. We want to use this info to update
		// node contact info and do not want to terminate execution by returning an err
		mon.Event("failed dial")
		pingNodeSuccess = false
		pingErrorMessage = fmt.Sprintf("failed to dial storage node (ID: %s) at address %s: %q", peerID, req.Address, err)
		endpoint.log.Info("pingBack failed to dial storage node", zap.Stringer("Node ID", peerID), zap.String("node address", req.Address), zap.String("pingErrorMessage", pingErrorMessage), zap.Error(err))
		return pingNodeSuccess, pingErrorMessage, nil
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	_, err = client.pingNode(ctx, &pb.ContactPingRequest{})
	if err != nil {
		mon.Event("failed ping node")
		pingNodeSuccess = false
		pingErrorMessage = fmt.Sprintf("failed to ping storage node, your node indicated error code: %d, %q", rpcstatus.Code(err), err)
		endpoint.log.Info("pingBack pingNode error", zap.Stringer("Node ID", peerID), zap.String("pingErrorMessage", pingErrorMessage), zap.Error(err))
	}

	return pingNodeSuccess, pingErrorMessage, nil
}
