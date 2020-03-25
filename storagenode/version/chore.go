// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/storagenode/notifications"
)

var (
	mon = monkit.Package()
)

// Chore contains the information and variables to ensure the Software is up to date for storagenode.
type Chore struct {
	log     *zap.Logger
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
		suggested, err := chore.service.CheckVersion(ctx)
		if err != nil {
			notification := notifications.NewNotification{
				SenderID: chore.nodeID,
				Type:     notifications.TypeCustom,
				Title:    "It’s time to update your Node’s software, you are running outdated version " + chore.service.Info.Version.String(),
				Message:  "Failure to update your software soon will impact your reputation and payout amount because your Node could potentially be disqualified shortly",
			}

			_, err = chore.notifications.Receive(ctx, notification)
			if err != nil {
				chore.log.Sugar().Errorf("Failed to insert notification", err.Error())
			}
		}

		if chore.service.Info.Version.Compare(suggested) < 0 {
			notification := notifications.NewNotification{
				SenderID: chore.nodeID,
				Type:     notifications.TypeCustom,
				Title:    "Update your Node to Version " + suggested.String(),
				Message:  "It's time to update your Node's software, you are running outdated version " + chore.service.Info.Version.String(),
			}

			_, err = chore.notifications.Receive(ctx, notification)
			if err != nil {
				chore.log.Sugar().Errorf("Failed to insert notification", err.Error())
			}
		}

		return nil
	})
}
