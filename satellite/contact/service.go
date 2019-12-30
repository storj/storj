// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"fmt"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

// Config contains configurable values for contact service
type Config struct {
	ExternalAddress string `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`
}

// Service is the contact service between storage nodes and satellites.
// It is responsible for updating general node information like address, capacity, and uptime.
// It is also responsible for updating peer identity information for verifying signatures from that node.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	mutex sync.Mutex
	self  *overlay.NodeDossier

	overlay *overlay.Service
	peerIDs overlay.PeerIdentities
	dialer  rpc.Dialer
}

// NewService creates a new contact service.
func NewService(log *zap.Logger, self *overlay.NodeDossier, overlay *overlay.Service, peerIDs overlay.PeerIdentities, dialer rpc.Dialer) *Service {
	return &Service{
		log:     log,
		self:    self,
		overlay: overlay,
		peerIDs: peerIDs,
		dialer:  dialer,
	}
}

// Local returns the satellite node dossier
func (service *Service) Local() overlay.NodeDossier {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	return *service.self
}

// Close closes resources
func (service *Service) Close() error { return nil }

// PingBack pings the node to test connectivity.
func (service *Service) PingBack(ctx context.Context, address string, peerID storj.NodeID) (_ bool, _ string, err error) {
	defer mon.Task()(&ctx)(&err)

	pingNodeSuccess := true
	var pingErrorMessage string

	client, err := dialNode(ctx, service.dialer, address, peerID)
	if err != nil {
		// If there is an error from trying to dial and ping the node, return that error as
		// pingErrorMessage and not as the err. We want to use this info to update
		// node contact info and do not want to terminate execution by returning an err
		mon.Event("failed dial")
		pingNodeSuccess = false
		pingErrorMessage = fmt.Sprintf("failed to dial storage node (ID: %s) at address %s: %q", peerID, address, err)
		service.log.Info("pingBack failed to dial storage node", zap.Stringer("Node ID", peerID), zap.String("node address", address), zap.String("pingErrorMessage", pingErrorMessage), zap.Error(err))
		return pingNodeSuccess, pingErrorMessage, nil
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	_, err = client.pingNode(ctx, &pb.ContactPingRequest{})
	if err != nil {
		mon.Event("failed ping node")
		pingNodeSuccess = false
		pingErrorMessage = fmt.Sprintf("failed to ping storage node, your node indicated error code: %d, %q", rpcstatus.Code(err), err)
		service.log.Info("pingBack pingNode error", zap.Stringer("Node ID", peerID), zap.String("pingErrorMessage", pingErrorMessage), zap.Error(err))
	}

	return pingNodeSuccess, pingErrorMessage, nil
}
