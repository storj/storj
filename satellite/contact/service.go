// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/rpc"
	"storj.io/storj/satellite/overlay"
)

// Error is the default error class for contact package.
var Error = errs.Class("contact")

var mon = monkit.Package()

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
