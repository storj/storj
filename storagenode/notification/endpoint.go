// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
)

// Error is the default error class for notification package.
var Error = errs.Class("notification")

// Endpoint is
type Endpoint struct {
	log     *zap.Logger
	service *Service
}

// NewEndpoint creates a new notification endpoint.
func NewEndpoint(log *zap.Logger, service *Service) *Endpoint {
	return &Endpoint{
		log:     log,
		service: service,
	}
}

// ProcessNotification handles incoming notification messages
func (endpoint *Endpoint) ProcessNotification(ctx context.Context, message *pb.NotificationMessage) (_ *pb.NotificationResponse, err error) {
	switch message.Loglevel {
	case pb.LogLevel_INFO:
		endpoint.log.Info("Notification:", zap.String("message", string(message.Message)))
	case pb.LogLevel_WARN:
		endpoint.log.Warn("Notification:", zap.String("message", string(message.Message)))
	case pb.LogLevel_ERROR:
		endpoint.log.Error("Notification:", zap.String("message", string(message.Message)))
	case pb.LogLevel_DEBUG:
		endpoint.log.Debug("Notification:", zap.String("message", string(message.Message)))
	}
	return &pb.NotificationResponse{}, nil
}
