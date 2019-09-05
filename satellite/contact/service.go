// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"fmt"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/overlay"
)

// Error is the default error class for contact package
var Error = errs.Class("contact")

var mon = monkit.Package()

// Config contains configurable values for contact service
type Config struct {
	BatchSize int `help:"number of uptime update requests in the cache before we save them to the database" default:"1000"`
}

// Service is the contact service between storage nodes and satellites
type Service struct {
	log                 *zap.Logger
	overlaySvc          *overlay.Service
	transport           transport.Client
	batchSize           int
	mu                  sync.Mutex
	updateRequestsCache []*overlay.NodeCheckinInfo
}

// NewService creates a new contact service
func NewService(log *zap.Logger, overlaySvc *overlay.Service, transport transport.Client, batchSize int) *Service {
	return &Service{
		log:                 log,
		overlaySvc:          overlaySvc,
		transport:           transport,
		batchSize:           batchSize,
		updateRequestsCache: []*overlay.NodeCheckinInfo{},
	}
}

// AddUpdateToCache adds an uptime update request to the contact updateRequestsCache
func (c *Service) AddUpdateToCache(ctx context.Context, updateRequest *overlay.NodeCheckinInfo) error {
	c.mu.Lock()
	c.updateRequestsCache = append(c.updateRequestsCache, updateRequest)
	c.mu.Unlock()
	return c.SaveUpdatesToDB(ctx)
}

// SaveUpdatesToDB saves the values in the uptime update cache to the overlay
// database once the batch size has been reached
func (c *Service) SaveUpdatesToDB(ctx context.Context) error {
	c.mu.Lock()
	if len(c.updateRequestsCache) < c.batchSize {
		return nil
	}

	cacheCopy := make([]*overlay.NodeCheckinInfo, len(c.updateRequestsCache))
	copy(cacheCopy, c.updateRequestsCache)
	c.updateRequestsCache = []*overlay.NodeCheckinInfo{}
	c.mu.Unlock()

	failed, err := c.overlaySvc.BatchUpdateUptime(ctx, cacheCopy)
	if err != nil {
		return err
	}
	if len(failed) > 0 {
		c.log.Info(fmt.Sprintf("failed updating overlay uptime check: %v", failed))
	}
	return nil
}
