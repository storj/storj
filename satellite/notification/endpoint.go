// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"

	"storj.io/storj/pkg/pb"
)

type Endpoint struct {
	service *Service
}

func NewEndpoint(service *Service) *Endpoint {
	return &Endpoint{
		service: service,
	}
}

// ProcessNotification process notification by rpc.
func (endpoint *Endpoint) ProcessNotification(ctx context.Context, message *pb.NotificationMessage) (*pb.NotificationResponse, error) {
	nodeIDs, err := endpoint.service.overlay.ActiveLastWeek(ctx)
	if err != nil {
		return nil, err
	}

	var nodes []pb.Node
	for i := range nodeIDs {
		node, err := endpoint.service.overlay.Get(ctx, nodeIDs[i])
		if err != nil {
			return nil, Error.Wrap(err)
		}

		nodes = append(nodes, node.Node)
	}

	endpoint.service.sendBroadcastNotification(ctx, string(message.Message), nodes)

	return &pb.NotificationResponse{}, nil
}
