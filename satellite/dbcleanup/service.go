// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbcleanup

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/satellite/orders"
)

var (
	// Error the default dbcleanup errs class
	Error = errs.Class("dbcleanup error")

	mon = monkit.Package()
)

// Config defines configuration struct for dbcleanup service
type Config struct {
	SerialsInterval time.Duration `help:"how often to delete expired serial numbers" default:"24h"`
}

// Service for deleting DB entries that are no longer needed.
type Service struct {
	log    *zap.Logger
	orders orders.DB

	Serials sync2.Cycle
}

// NewService creates new service for deleting DB entries.
func NewService(log *zap.Logger, orders orders.DB, config Config) *Service {
	return &Service{
		log:    log,
		orders: orders,

		Serials: *sync2.NewCycle(config.SerialsInterval),
	}
}

// Run starts the db cleanup service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.Serials.Run(ctx, service.deleteExpiredSerials)
}

func (service *Service) deleteExpiredSerials(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Debug("deleting expired serial numbers")

	deleted, err := service.orders.DeleteExpiredSerials(ctx)
	if err != nil {
		service.log.Error("deleting expired serial numbers", zap.Error(err))
		return nil
	}

	service.log.Debug("expired serials deleted", zap.Int("items deleted", deleted))
	return nil
}

// Close stops the dbcleanup service
func (service *Service) Close() error {
	service.Serials.Close()
	return nil
}
