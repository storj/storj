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

// ProcessNotification sends notification to node by ID.
func (endpoint *Endpoint) ProcessNotification(ctx context.Context, message *pb.Notification, id storj.NodeID) (_ *pb.NotificationResponse, err error) {
	address, err := endpoint.service.db.GetAddressByID(ctx, id)
	return endpoint.service.ProcessNotification(ctx, message, id, address)
}

// ProcessNotifications sends group of notifications to node by ID.
func (endpoint *Endpoint) ProcessNotifications(ctx context.Context, message []*pb.Notification, id storj.NodeID) (_ []*pb.NotificationResponse, err error) {
	address, err := endpoint.service.db.GetAddressByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return endpoint.service.ProcessNotifications(ctx, message, id, address)
}
