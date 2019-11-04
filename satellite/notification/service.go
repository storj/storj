// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
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
	log     *zap.Logger
	config  Config
	dialer  rpc.Dialer
	overlay overlay.DB
	mailer  *mailservice.Service

	loop    *sync2.Cycle
	lock    *sync.Mutex
	limiter map[string]ClientSetting
}

// NewService creates a new notification service.
func NewService(log *zap.Logger, config Config, dialer rpc.Dialer, overlay overlay.DB, mail *mailservice.Service) *Service {
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

// Run runs a notification cycle every 5 Seconds
func (service *Service) Run(ctx context.Context) error {
	service.log.Debug("Starting Loop")
	service.loop = sync2.NewCycle(time.Second * 5)

	_ = service.loop.Run(ctx, service.debug)

	return nil
}

// debug sends a dumb notification
func (service *Service) debug(ctx context.Context) error {
	// TODO: Get all nodes from the DB
	nodem := pb.NotificationMessage{
		NodeId:   pb.NodeID{},
		Address:  "localhost:13000",
		Loglevel: pb.LogLevel_INFO,
		Message:  []byte("Hello Node"),
	}
	service.log.Info("Debug Message")
	client, err := newClient(ctx, service.dialer, nodem.Address, nodem.NodeId)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, client.Close()) }()
	_, err = client.client.ProcessNotification(ctx, &nodem)
	if err != nil {
		return err
	}
	return nil
}

// Close closes resources
func (service *Service) Close() error {

	service.loop.Stop()
	service.loop.Close()

	return nil
}

func (service *Service) IncrementLimiter(id string, email bool, rpc bool) {
	service.lock.Lock()
	entry := service.limiter[id]
	if email {
		entry.Emails++
	}
	if rpc {
		entry.RPC++
	}
	service.limiter[id] = entry
	service.lock.Unlock()
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
