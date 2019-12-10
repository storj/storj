// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

var (
	// Error is the default error class for notification package.
	Error = errs.Class("notification")

	mon = monkit.Package()
)

// Service is the notification service between storage nodes and satellites.
// architecture: Service
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer
	db     NotificationDB
}

// NewService creates a new notification service.
func NewService(log *zap.Logger, dialer rpc.Dialer) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
	}
}

// ProcessNotification sends message to the specified node.
func (service *Service) ProcessNotification(ctx context.Context, message *pb.Notification, id storj.NodeID, address string) (_ *pb.NotificationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Debug("sending to node", zap.String("address", address), zap.String("message", string(message.Message)))
	result, err := service.processNotification(ctx, message, id, address)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ProcessNotifications sends messages to the specified node.
func (service *Service) ProcessNotifications(ctx context.Context, messages []*pb.Notification, id storj.NodeID, address string) (_ []*pb.NotificationResponse, err error) {
	var sentCount int
	var result []*pb.NotificationResponse

	for i := range messages {
		// RPC Message
		mess := &pb.Notification{
			Message:          messages[i].Message,
			Title:            messages[i].Title,
			NotificationType: messages[i].NotificationType,
		}

		res, err := service.ProcessNotification(ctx, mess, id, address)
		if err != nil {
			return nil, err
		}

		result = append(result, res)
	}

	service.log.Debug("sent to nodes", zap.Int("count", sentCount))
	return result, nil
}

// processNotification sends message to the specified node.
func (service *Service) processNotification(ctx context.Context, message *pb.Notification, id storj.NodeID, address string) (_ *pb.NotificationResponse, err error) {
	client, err := newClient(ctx, service.dialer, address, id)
	if err != nil {
		_, ok := err.(net.Error)
		if ok {
			return &pb.NotificationResponse{}, err
		}

		service.log.Warn("internal error", zap.String("error", err.Error()))
		return &pb.NotificationResponse{}, err
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	return client.client.ProcessNotification(ctx, message)
}
