// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/private/nodeoperator"
	"storj.io/storj/satellite/overlay"
)

var (
	errPingBackDial     = errs.Class("pingback dialing")
	errCheckInIdentity  = errs.Class("check-in identity")
	errCheckInRateLimit = errs.Class("check-in ratelimit")
	errCheckInNetwork   = errs.Class("check-in network")
)

// Endpoint implements the contact service Endpoints.
type Endpoint struct {
	pb.DRPCNodeUnimplementedServer
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

	// we need a string as a key for the limiter, but nodeID.String() has base58 encoding overhead
	nodeIDBytesAsString := string(nodeID.Bytes())
	if !endpoint.service.idLimiter.IsAllowed(nodeIDBytesAsString) {
		endpoint.log.Info("node rate limited by id", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID))
		return nil, rpcstatus.Error(rpcstatus.ResourceExhausted, errCheckInRateLimit.New("node rate limited by id").Error())
	}

	err = endpoint.service.peerIDs.Set(ctx, nodeID, peerID)
	if err != nil {
		endpoint.log.Info("failed to add peer identity entry for ID", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.FailedPrecondition, errCheckInIdentity.New("failed to add peer identity entry for ID: %v", err).Error())
	}

	resolvedIPPort, resolvedNetwork, err := overlay.ResolveIPAndNetwork(ctx, req.Address)
	if err != nil {
		endpoint.log.Info("failed to resolve IP from address", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, errCheckInNetwork.New("failed to resolve IP from address: %s, err: %v", req.Address, err).Error())
	}

	nodeurl := storj.NodeURL{
		ID:      nodeID,
		Address: req.Address,
	}
	pingNodeSuccess, pingNodeSuccessQUIC, pingErrorMessage, err := endpoint.service.PingBack(ctx, nodeurl)
	if err != nil {
		return nil, endpoint.checkPingRPCErr(err, nodeurl)
	}

	// check wallet features
	if req.Operator != nil {
		if err := nodeoperator.DefaultWalletFeaturesValidation.Validate(req.Operator.WalletFeatures); err != nil {
			endpoint.log.Debug("ignoring invalid wallet features",
				zap.Stringer("Node ID", nodeID),
				zap.Strings("Wallet Features", req.Operator.WalletFeatures))

			// TODO: Update CheckInResponse to include wallet feature validation error
			req.Operator.WalletFeatures = nil
		}
	}

	nodeInfo := overlay.NodeCheckInInfo{
		NodeID: peerID.ID,
		Address: &pb.NodeAddress{
			Address:   req.Address,
			Transport: pb.NodeTransport_TCP_TLS_GRPC,
		},
		LastNet:    resolvedNetwork,
		LastIPPort: resolvedIPPort,
		IsUp:       pingNodeSuccess,
		Capacity:   req.Capacity,
		Operator:   req.Operator,
		Version:    req.Version,
	}

	err = endpoint.service.overlay.UpdateCheckIn(ctx, nodeInfo, time.Now().UTC())
	if err != nil {
		endpoint.log.Info("failed to update check in", zap.String("node address", req.Address), zap.Stringer("Node ID", nodeID), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, Error.Wrap(err).Error())
	}

	endpoint.log.Debug("checking in", zap.Stringer("Node ID", nodeID), zap.String("node addr", req.Address), zap.Bool("ping node success", pingNodeSuccess), zap.String("ping node err msg", pingErrorMessage))
	return &pb.CheckInResponse{
		PingNodeSuccess:     pingNodeSuccess,
		PingNodeSuccessQuic: pingNodeSuccessQUIC,
		PingErrorMessage:    pingErrorMessage,
	}, nil
}

// GetTime returns current timestamp.
func (endpoint *Endpoint) GetTime(ctx context.Context, req *pb.GetTimeRequest) (_ *pb.GetTimeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Info("failed to get node ID from context", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, errCheckInIdentity.New("failed to get ID from context: %v", err).Error())
	}

	currentTimestamp := time.Now().UTC()
	endpoint.log.Debug("get system current time", zap.Stringer("timestamp", currentTimestamp), zap.Stringer("node id", peerID.ID))
	return &pb.GetTimeResponse{
		Timestamp: currentTimestamp,
	}, nil
}

// PingMe is called by storage node to request a pingBack from the satellite to confirm they can
// successfully connect to the node.
func (endpoint *Endpoint) PingMe(ctx context.Context, req *pb.PingMeRequest) (_ *pb.PingMeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peerID, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		endpoint.log.Info("failed to get node ID from context", zap.String("node address", req.Address), zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Unknown, errCheckInIdentity.New("failed to get ID from context: %v", err).Error())
	}
	nodeID := peerID.ID

	nodeURL := storj.NodeURL{
		ID:      nodeID,
		Address: req.Address,
	}

	if endpoint.service.timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, endpoint.service.timeout)
		defer cancel()
	}

	switch req.Transport {

	case pb.NodeTransport_QUIC_GRPC:
		err = endpoint.service.pingNodeQUIC(ctx, nodeURL)
		if err != nil {
			return nil, endpoint.checkPingRPCErr(err, nodeURL)
		}
		return &pb.PingMeResponse{}, nil

	case pb.NodeTransport_TCP_TLS_GRPC:
		client, err := dialNodeURL(ctx, endpoint.service.dialer, nodeURL)
		if err != nil {
			return nil, endpoint.checkPingRPCErr(err, nodeURL)
		}

		defer func() { err = errs.Combine(err, client.Close()) }()

		_, err = client.pingNode(ctx, &pb.ContactPingRequest{})
		if err != nil {
			return nil, endpoint.checkPingRPCErr(err, nodeURL)
		}
		return &pb.PingMeResponse{}, nil
	}

	return nil, rpcstatus.Errorf(rpcstatus.InvalidArgument, "invalid transport: %v", req.Transport)
}

func (endpoint *Endpoint) checkPingRPCErr(err error, nodeURL storj.NodeURL) error {
	endpoint.log.Info("failed to ping back address", zap.String("node address", nodeURL.Address), zap.Stringer("Node ID", nodeURL.ID), zap.Error(err))
	if errPingBackDial.Has(err) {
		err = errCheckInNetwork.New("failed dialing address when attempting to ping node (ID: %s): %s, err: %v", nodeURL.ID, nodeURL.Address, err)
		return rpcstatus.Error(rpcstatus.NotFound, err.Error())
	}
	err = errCheckInNetwork.New("failed to ping node (ID: %s) at address: %s, err: %v", nodeURL.ID, nodeURL.Address, err)
	return rpcstatus.Error(rpcstatus.NotFound, err.Error())
}
