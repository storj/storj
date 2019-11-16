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

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/sap"
	"storj.io/storj/private/sync2"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/overlay"
)

type ClientSetting struct {
	Emails    int
	RPC       int
	lastReset time.Time
}

// Service is the notification service between storage nodes and satellites.
// architecture: Service
type Service struct {
	log     sap.Logger
	config  Config
	dialer  rpc.Dialer
	overlay *overlay.Service
	mailer  *mailservice.Service

	loop    *sync2.Cycle
	lock    *sync.Mutex
	limiter map[string]ClientSetting
}

// NewService creates a new notification service.
func NewService(log sap.Logger, config Config, dialer rpc.Dialer, overlay *overlay.Service, mail *mailservice.Service) *Service {
	return &Service{
		log:     log,
		config:  config,
		dialer:  dialer,
		overlay: overlay,
		mailer:  mail,
		limiter: map[string]ClientSetting{},
		lock:    &sync.Mutex{},
	}
}

// Run sets the Rate Limiter up to ensure we dont spam
func (service *Service) Run(ctx context.Context) (err error) {
	service.log.Debug("Starting Rate Limiter")
	service.loop = sync2.NewCycle(1 * time.Hour)

	err = service.loop.Run(ctx, service.resetLimiter)

	return err
}

// resetLimiter resets the Usage every hour
func (service *Service) resetLimiter(ctx context.Context) error {
	service.lock.Lock()
	defer service.lock.Unlock()
	//Clear map
	service.limiter = map[string]ClientSetting{}
	return nil
}

// Close closes the resources
func (service *Service) Close() error {

	service.loop.Stop()
	service.loop.Close()

	return nil
}

func (service *Service) IncrementLimiter(id string, email bool, rpc bool) {
	service.lock.Lock()
	defer service.lock.Unlock()
	entry := service.limiter[id]
	if email {
		entry.Emails++
	}
	if rpc {
		entry.RPC++
	}
	service.limiter[id] = entry
}

func (service *Service) CheckRPCLimit(id string) bool {
	if entry, ok := service.limiter[id]; ok && entry.RPC < service.config.HourlyRPC {
		return false
	}
	return true
}

func (service *Service) CheckEmailLimit(id string) bool {
	if entry, ok := service.limiter[id]; ok && entry.Emails < service.config.HourlyEmails {
		return false
	}
	return true
}

// ProcessNotification sends message to the specified set of nodes (ids)
func (service *Service) ProcessNotification(message *pb.NotificationMessage) (err error) {
	var eSent, rSent = false, false
	ctx := context.Background()
	service.log.Debug("sending to node", zap.String("address", message.Address), zap.String("message", string(message.Message)))
	if service.CheckRPCLimit(message.NodeId.String()) {
		_, err = service.processNotificationRPC(ctx, message)
		if err != nil {
			return err
		}
		rSent = true
	}
	if service.CheckEmailLimit(message.NodeId.String()) {
		err = service.processNotificationEmail(ctx, message)
		if err != nil {
			return err
		}
		eSent = true
	}
	service.IncrementLimiter(message.NodeId.String(), eSent, rSent)
	return nil
}

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

func (service *Service) processNotificationEmail(ctx context.Context, message *pb.NotificationMessage) (err error) {
	//return endpoint.service.mailer.Send(ctx, &post.Message{})
	return nil
}

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

		err := service.ProcessNotification(mess)
		if err != nil {
			failed = append(failed, node.Id.String())
		}
		sentCount++
	}

	service.log.Info("sent to nodes", zap.Int("count", sentCount))
	service.log.Debug("notification to the following nodes failed", zap.Strings("nodeIDs", failed))
}
