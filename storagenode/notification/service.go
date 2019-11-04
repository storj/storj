// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"go.uber.org/zap"

	"storj.io/storj/pkg/rpc"
)

// Service is the notification service between storage nodes and satellites.
// architecture: Service
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer
}

// NewService creates a new notification service.
func NewService(log *zap.Logger, dialer rpc.Dialer) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
	}
}

// Close closes resources
func (service *Service) Close() error { return nil }
