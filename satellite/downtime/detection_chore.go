// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/overlay"
)

// DetectionChore looks for nodes that have not checked in and tries to contact them.
//
// architecture: Chore
type DetectionChore struct {
	log     *zap.Logger
	Loop    sync2.Cycle
	config  Config
	overlay *overlay.Service
	service *Service
	db      DB
}

// NewDetectionChore instantiates DetectionChore.
func NewDetectionChore(log *zap.Logger, config Config, overlay *overlay.Service, service *Service, db DB) *DetectionChore {
	return &DetectionChore{
		log:     log,
		Loop:    *sync2.NewCycle(config.DetectionInterval),
		config:  config,
		overlay: overlay,
		service: service,
		db:      db,
	}
}

// Run starts the chore.
func (chore *DetectionChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		chore.log.Debug("checking for nodes that have not had a successful check-in within the interval.",
			zap.Stringer("interval", chore.config.DetectionInterval))

		nodeLastContacts, err := chore.overlay.GetSuccesfulNodesNotCheckedInSince(ctx, chore.config.DetectionInterval)
		if err != nil {
			chore.log.Error("error retrieving node addresses for downtime detection.", zap.Error(err))
			return nil
		}
		chore.log.Debug("nodes that have had not had a successful check-in with the interval.",
			zap.Stringer("interval", chore.config.DetectionInterval),
			zap.Int("count", len(nodeLastContacts)))

		for _, nodeLastContact := range nodeLastContacts {
			success, err := chore.service.CheckAndUpdateNodeAvailability(ctx, nodeLastContact.ID, nodeLastContact.Address)
			if err != nil {
				chore.log.Error("error during downtime detection ping back.",
					zap.Bool("success", success),
					zap.Error(err))

				continue
			}

			if !success {
				now := time.Now().UTC()
				duration := now.Sub(nodeLastContact.LastContactSuccess) - chore.config.DetectionInterval

				err = chore.db.Add(ctx, nodeLastContact.ID, now, duration)
				if err != nil {
					chore.log.Error("error adding node seconds offline information.",
						zap.Stringer("node ID", nodeLastContact.ID),
						zap.Stringer("duration", duration),
						zap.Error(err))
				}
			}
		}
		return nil
	})
}

// Close closes chore.
func (chore *DetectionChore) Close() error {
	chore.Loop.Close()
	return nil
}
