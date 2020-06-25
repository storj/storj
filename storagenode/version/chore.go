// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/storagenode/notifications"
)

var (
	mon = monkit.Package()
)

// Chore contains the information and variables to ensure the Software is up to date for storagenode.
type Chore struct {
	service *checker.Service

	Loop          *sync2.Cycle
	nodeID        storj.NodeID
	notifications *notifications.Service
}

// NewChore creates a Version Check Client with default configuration for storagenode.
func NewChore(service *checker.Service, notifications *notifications.Service, nodeID storj.NodeID, checkInterval time.Duration) *Chore {
	return &Chore{
		service:       service,
		nodeID:        nodeID,
		notifications: notifications,
		Loop:          sync2.NewCycle(checkInterval),
	}
}

// Run logs the current version information
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.service.Checked() {
		_, err = chore.service.CheckVersion(ctx)
		if err != nil {
			return err
		}
	}

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		_, _ = chore.service.CheckVersion(ctx)

		return nil
	})
}
