// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package bandwidth implements bandwidth usage rollup loop.
package bandwidth

import (
	"context"
	"time"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/sync2"
)

var mon = monkit.Package()

// Service implements
type Service struct {
	log  *zap.Logger
	db   DB
	Loop sync2.Cycle
}

// NewService creates a new bandwidth service.
func NewService(log *zap.Logger, db DB) *Service {
	return &Service{
		log:  log,
		db:   db,
		Loop: *sync2.NewCycle(time.Hour * 1),
	}
}

// Run starts the background process for rollups of bandwidth usage
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return service.Loop.Run(ctx, service.db.Rollup)
}

// Close stops the background process for rollups of bandwidth usage
func (service *Service) Close() (err error) {
	service.Loop.Close()
	return nil
}
