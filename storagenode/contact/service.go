// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// Service is the contact service between storage nodes and satellites
type Service struct {
	log       *zap.Logger
	self      *overlay.NodeDossier
	transport transport.Client

	mu               sync.Mutex
	lastPinged       time.Time
	whoPingedNodeID  storj.NodeID
	whoPingedAddress string
}

// NewService creates a new contact service
func NewService(log *zap.Logger, self *overlay.NodeDossier, transport transport.Client) *Service {
	return &Service{
		log:       log,
		self:      self,
		transport: transport,
	}
}

// LastPinged returns last time someone pinged this node.
func (service *Service) whenLastPinged() (when time.Time, who storj.NodeID, addr string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.lastPinged, service.whoPingedNodeID, service.whoPingedAddress
}

// Pinged notifies the service it has been remotely pinged.
func (service *Service) wasPinged(when time.Time, srcNodeID storj.NodeID, srcAddress string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.lastPinged = when
	service.whoPingedNodeID = srcNodeID
	service.whoPingedAddress = srcAddress
}

// Local returns the local node
func (service *Service) Local() overlay.NodeDossier {
	return *service.self
}
