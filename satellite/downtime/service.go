// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/contact"
	"storj.io/storj/satellite/overlay"
)

// Service is a service for downtime checking.
//
// architecture: Service
type Service struct {
	log     *zap.Logger
	overlay *overlay.Service
	contact *contact.Service
}

// NewService creates a new downtime tracking service.
func NewService(log *zap.Logger, overlay *overlay.Service, contact *contact.Service) *Service {
	return &Service{
		log:     log,
		overlay: overlay,
		contact: contact,
	}
}

// CheckAndUpdateNodeAvailability tries to ping the supplied address and updates the uptime based on ping success or failure. Returns true if the ping and uptime updates are successful.
func (service *Service) CheckAndUpdateNodeAvailability(ctx context.Context, nodeID storj.NodeID, address string) (success bool, err error) {
	defer mon.Task()(&ctx)(&err)

	pingNodeSuccess, pingErrorMessage, err := service.contact.PingBack(ctx, address, nodeID)
	if err != nil {
		service.log.Error("error during downtime detection ping back.",
			zap.String("ping error", pingErrorMessage),
			zap.Error(err))

		return false, errs.Wrap(err)
	}

	if pingNodeSuccess {
		_, err = service.overlay.UpdateUptime(ctx, nodeID, true)
		if err != nil {
			service.log.Error("error updating node contact success information.",
				zap.Stringer("node ID", nodeID),
				zap.Error(err))

			return false, errs.Wrap(err)
		}

		return true, nil
	}

	_, err = service.overlay.UpdateUptime(ctx, nodeID, false)
	if err != nil {
		service.log.Error("error updating node contact failure information.",
			zap.Stringer("node ID", nodeID),
			zap.Error(err))

		return false, errs.Wrap(err)
	}

	return false, nil
}

// Close closes resources
func (service *Service) Close() error { return nil }
