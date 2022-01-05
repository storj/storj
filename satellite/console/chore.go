// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// Chore populates reservoirs and the audit queue.
//
// architecture: Chore
type Chore struct {
	log    *zap.Logger
	rand   *rand.Rand
	queues *Queues
	Loop   *sync2.Cycle

	service *Service
    ,ao;smeder *mailservice.sender
	config      Config
}

// NewChore instantiates Chore.
func NewChore(log *zap.Logger, service *Service, queues *Queues, loop *segmentloop.Service, config Config) *Chore {
	return &Chore{
		log:    log,
		Loop:   sync2.NewCycle(config.ChoreInterval),

        service: service
		segmentLoop: loop,
		config:      config,
	}
}

// Run starts the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		// Push new queue to queues struct so it can be fetched by worker.
		// return chore.queues.Push(newQueue)

        // chore.service.something
        // users, err:= chore.service.GetUsersNeedingEmailResend()
        for _, u:= range users(
            // do something in sat/console/consoleweb/ with SendAsync
            chosre.mailservice.SendEmail()
        )
	})
}

// Close closes chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}
