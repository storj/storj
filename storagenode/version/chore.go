// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/private/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/storagenode/notifications"
)

var (
	mon = monkit.Package()
)

// Relevance contains information about software being outdated.
type Relevance struct {
	expectedVersion  version.SemVer
	isOutdated       bool
	firstTimeSpotted time.Time
	timesNotified    notifications.TimesNotified
}

// Chore contains the information and variables to ensure the Software is up to date for storagenode.
type Chore struct {
	log     *zap.Logger
	service *checker.Service

	Loop          *sync2.Cycle
	nodeID        storj.NodeID
	notifications *notifications.Service

	version Relevance
}

// NewChore creates a Version Check Client with default configuration for storagenode.
func NewChore(log *zap.Logger, service *checker.Service, notifications *notifications.Service, nodeID storj.NodeID, checkInterval time.Duration) *Chore {
	return &Chore{
		log:           log,
		service:       service,
		nodeID:        nodeID,
		notifications: notifications,
		Loop:          sync2.NewCycle(checkInterval),
	}
}

// Run logs the current version information and detects if software outdated, if so - sends notifications.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.service.Checked() {
		_, err = chore.service.CheckVersion(ctx)
		if err != nil {
			return err
		}
	}

	chore.version.init(chore.service.Info.Version)

	now := time.Now().UTC()

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		suggested, err := chore.service.CheckVersion(ctx)
		if err != nil {
			return err
		}

		chore.version.checkRelevance(suggested, chore.service.Info.Version)

		if !chore.version.isOutdated {
			return nil
		}

		var notification notifications.NewNotification

		switch {
		case chore.version.firstTimeSpotted.Add(time.Hour*335).Before(now) && chore.version.timesNotified == notifications.TimesNotifiedSecond:
			notification = notifications.NewVersionNotification(notifications.TimesNotifiedSecond, suggested, chore.nodeID)
			chore.version.timesNotified = notifications.TimesNotifiedLast

		case chore.version.firstTimeSpotted.Add(time.Hour*144).Before(now) && chore.version.timesNotified == notifications.TimesNotifiedFirst:
			notification = notifications.NewVersionNotification(notifications.TimesNotifiedFirst, suggested, chore.nodeID)
			chore.version.timesNotified = notifications.TimesNotifiedSecond

		case chore.version.firstTimeSpotted.Add(time.Hour*96).Before(now) && chore.version.timesNotified == notifications.TimesNotifiedZero:
			notification = notifications.NewVersionNotification(notifications.TimesNotifiedZero, suggested, chore.nodeID)
			chore.version.timesNotified = notifications.TimesNotifiedFirst
		default:
			return nil
		}

		_, err = chore.notifications.Receive(ctx, notification)
		if err != nil {
			chore.log.Sugar().Errorf("Failed to receive notification", err.Error())
		}

		return nil
	})
}

func (relevance *Relevance) init(currentVer version.SemVer) {
	relevance.expectedVersion = currentVer
	relevance.firstTimeSpotted = time.Now().UTC()
	relevance.timesNotified = notifications.TimesNotifiedZero
}

func (relevance *Relevance) checkRelevance(suggested version.SemVer, current version.SemVer) {
	if current.Compare(suggested) < 0 {
		relevance.isOutdated = true
		if relevance.expectedVersion.Compare(suggested) < 0 {
			relevance.expectedVersion = suggested
			relevance.firstTimeSpotted = time.Now().UTC()
			relevance.timesNotified = notifications.TimesNotifiedZero
		}
	} else {
		relevance.isOutdated = false
		relevance.timesNotified = notifications.TimesNotifiedZero
	}
}
