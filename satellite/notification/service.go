// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
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

// Close closes resources
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
