// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/pb"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/overlay"
)

// Error is the default error class for contact package
var Error = errs.Class("contact")

var mon = monkit.Package()

// Config contains configurable values for contact service
type Config struct {
	ExternalAddress string `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`

	// Chore config values
	Interval time.Duration `help:"how frequently the node contact chore should run" releaseDefault:"1h" devDefault:"30s"`
}

// Service is the contact service between storage nodes and satellites
type Service struct {
	log *zap.Logger

	mu   sync.Mutex
	self *overlay.NodeDossier

	initialized sync2.Fence
}

// NewService creates a new contact service
func NewService(log *zap.Logger, self *overlay.NodeDossier) *Service {
	return &Service{
		log:  log,
		self: self,
	}
}

// Local returns the storagenode node-dossier
func (service *Service) Local() overlay.NodeDossier {
	service.mu.Lock()
	defer service.mu.Unlock()
	return *service.self
}

// UpdateSelf updates the local node with the capacity
func (service *Service) UpdateSelf(capacity *pb.NodeCapacity) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if capacity != nil {
		service.self.Capacity = *capacity
	}

	service.initialized.Release()
}
