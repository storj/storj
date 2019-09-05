// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/overlay"
)

// Error is the default error class for contact package
var Error = errs.Class("contact")

var mon = monkit.Package()

// Config contains configurable values for contact service
type Config struct {
	ExternalAddress string `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`
}

// Service is the contact service between storage nodes and satellites
//
// architecture: Service
type Service struct {
	log       *zap.Logger
	self      overlay.NodeDossier
	overlay   *overlay.Service
	transport transport.Client
}

// NewService creates a new contact service
func NewService(log *zap.Logger, self overlay.NodeDossier, overlay *overlay.Service, transport transport.Client) *Service {
	return &Service{
		log:       log,
		self:      self,
		overlay:   overlay,
		transport: transport,
	}
}

// Local returns the satellite node dossier
func (service *Service) Local() overlay.NodeDossier {
	return service.self
}
