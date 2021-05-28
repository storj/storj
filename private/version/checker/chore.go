// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
	"time"

	"storj.io/common/sync2"
)

// Chore contains the information and variables to ensure the Software is up to date.
type Chore struct {
	service *Service

	Loop *sync2.Cycle
}

// NewChore creates a Version Check Client with default configuration.
func NewChore(service *Service, checkInterval time.Duration) *Chore {
	return &Chore{
		service: service,
		Loop:    sync2.NewCycle(checkInterval),
	}
}

// Run logs the current version information.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if !chore.service.Checked() {
		_, err := chore.service.CheckVersion(ctx)
		if err != nil {
			return err
		}
	}
	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		chore.service.checkVersion(ctx)
		return nil
	})
}
