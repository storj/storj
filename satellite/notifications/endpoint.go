// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Endpoint is the handler for notification messages
type Endpoint struct {
	service *Service
}

// NewEndpoint creates a new notification endpoint.
func NewEndpoint(service *Service) *Endpoint {
	return &Endpoint{
		service: service,
	}
}

func (endpoint *Endpoint) ProcessNotification(ctx context.Context, message *pb.Notification, id storj.NodeID) (_ *pb.NotificationResponse, err error) {
	// return endpoint.service.processNotification(ctx, message, id, address)
	// TODO: get adress from ID with interface
	return
}

func (endpoint *Endpoint) ProcessNotifications(ctx context.Context, message []*pb.Notification, id storj.NodeID) {
	//endpoint.service.ProcessNotifications(ctx, message, id, address)
	// TODO: get adress from ID with interface
}
