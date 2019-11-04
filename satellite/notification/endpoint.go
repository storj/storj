// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
)

// Endpoint is the rpc handler for the notification system
type Endpoint struct {
	log     *zap.Logger
	service *Service
}

// drpcEndpoint wraps streaming methods so that they can be used with drpc
type drpcEndpoint struct{ *Endpoint }

// NewEndpoint creates a new notification endpoint.
func NewEndpoint(log *zap.Logger, service *Service) *Endpoint {
	return &Endpoint{
		log:     log,
		service: service,
	}
}

// DRPC returns a DRPC form of the endpoint.
func (endpoint *Endpoint) DRPC() pb.DRPCNotificationServer {
	return &drpcEndpoint{Endpoint: endpoint}
}

// ProcessNotification sends message to the specified set of nodes (ids)
func (endpoint *Endpoint) ProcessNotification(ctx context.Context, message *pb.NotificationMessage) (_ *pb.NotificationResponse, err error) {
	endpoint.log.Info("Sending Notification to node", zap.String("address", message.Address), zap.String("message", string(message.Message)))
	if endpoint.service.CheckRPCLimit(message.NodeId.String()) {

		client, err := newClient(ctx, endpoint.service.dialer, message.Address, message.NodeId)
		if err != nil {
			// if this is a network error, then return the error otherwise just report internal error
			_, ok := err.(net.Error)
			if ok {
				return &pb.NotificationResponse{}, Error.New("failed to connect to %s: %v", message.Address, err)
			}
			endpoint.log.Info("notification internal error", zap.String("error", err.Error()))
			return &pb.NotificationResponse{}, Error.New("couldn't connect to client at addr: %s due to internal error.", message.Address)
		}
		defer func() { err = errs.Combine(err, client.Close()) }()

		return client.client.ProcessNotification(ctx, message, nil)
	}

	return &pb.NotificationResponse{}, nil
}

func (endpoint *Endpoint) sendBroadcastNotification(ctx context.Context, message string, ids []pb.Node) {
	var sentCount int
	var failed []string

	for _, node := range ids {
		// RPC Message
		mess := &pb.NotificationMessage{
			NodeId:   node.Id,
			Address:  node.Address.Address,
			Loglevel: pb.LogLevel_INFO,
			Message:  []byte(message),
		}

		_, err := endpoint.ProcessNotification(ctx, mess)
		if err != nil {
			failed = append(failed, node.Id.String())
		}
		sentCount++
	}

	endpoint.log.Info("Sent Notification to nodes", zap.Int("count", sentCount))
	//endpoint.log.Debug("Notification to the following nodes failed", zap.Array("nodeIDs", failed))
}
