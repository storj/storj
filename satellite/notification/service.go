// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/private/sync2"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/overlay"
)

var (
	// Error is the default error class for notification package.
	Error = errs.Class("notification")

	mon = monkit.Package()
)

// Service is the notification service between storage nodes and satellites.
// architecture: Service
type Service struct {
	log     *zap.Logger
	dialer  rpc.Dialer
	overlay *overlay.Service
	mailer  *mailservice.Service

	loop *sync2.Cycle
	lock *sync.Mutex
}

// NewService creates a new notification service.
func NewService(log *zap.Logger, dialer rpc.Dialer, overlay *overlay.Service, mail *mailservice.Service) *Service {
	return &Service{
		log:     log,
		dialer:  dialer,
		overlay: overlay,
		mailer:  mail,
		lock:    &sync.Mutex{},
	}
}

// Run sets the Rate Limiter up to ensure we dont spam.
func (service *Service) Run(ctx context.Context) (err error) {
	service.log.Debug("Starting Rate Limiter")
	service.loop = sync2.NewCycle(1 * time.Hour)

	err = service.loop.Run(ctx, nil)
	return err
}

// Close closes the resources
func (service *Service) Close() error {
	service.loop.Stop()
	service.loop.Close()

	return nil
}

// ProcessNotification sends message to the specified set of nodes (ids).
func (service *Service) ProcessNotification(ctx context.Context, message *pb.NotificationMessage) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Debug("sending to node", zap.String("address", message.Address), zap.String("message", string(message.Message)))
	_, err = service.processNotificationRPC(ctx, message)
	if err != nil {
		return err
	}

	err = service.processNotificationEmail(ctx, message)
	if err != nil {
		return err
	}

	return nil
}

// processNotificationRPC processing notification by rpc.
func (service *Service) processNotificationRPC(ctx context.Context, message *pb.NotificationMessage) (_ *pb.NotificationResponse, err error) {
	client, err := newClient(ctx, service.dialer, message.Address, message.NodeId)
	if err != nil {
		// if this is a network error, then return the error otherwise just report internal error
		_, ok := err.(net.Error)
		if ok {
			return &pb.NotificationResponse{}, Error.New("failed to connect to %s: %v", message.Address, err)
		}

		service.log.Warn("internal error", zap.String("error", err.Error()))
		return &pb.NotificationResponse{}, Error.New("couldn't connect to client at addr: %s due to internal error.", message.Address)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	return client.client.ProcessNotification(ctx, message)
}

// processNotificationEmail processing notification by mail service.
func (service *Service) processNotificationEmail(ctx context.Context, message *pb.NotificationMessage) (err error) {
	//return endpoint.service.mailer.Send(ctx, &post.Message{})
	return nil
}

//
func (service *Service) sendBroadcastNotification(ctx context.Context, message string, ids []pb.Node) {
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

		err := service.ProcessNotification(ctx, mess)
		if err != nil {
			failed = append(failed, node.Id.String())
		}
		sentCount++
	}
	service.log.Info("sent to nodes", zap.Int("count", sentCount))
	service.log.Debug("notification to the following nodes failed", zap.Strings("nodeIDs", failed))
}
