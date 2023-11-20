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
	"storj.io/common/version"
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

	version Relevance
	// nowFn used to mock time is tests.
	nowFn func() time.Time
}

// NewChore creates a Version Check Client with default configuration for storagenode.
func NewChore(log *zap.Logger, service *checker.Service, notifications *notifications.Service, nodeID storj.NodeID, checkInterval time.Duration) *Chore {
	chore := &Chore{
		log:           log,
		service:       service,
		nodeID:        nodeID,
		notifications: notifications,
		Loop:          sync2.NewCycle(checkInterval),
		nowFn:         time.Now().UTC,
	}

	chore.version.init(service.Info.Version)
	return chore
}

// Run logs the current version information and detects if software outdated, if so - sends notifications.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.service.Checked() {
		_, err = chore.checkVersion(ctx)
		if err != nil {
			return err
		}
	}

	return chore.Loop.Run(ctx, chore.RunOnce)
}

// RunOnce is like Run but only runs once.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	suggested, err := chore.checkVersion(ctx)
	if err != nil {
		return err
	}

	err = chore.checkRelevance(ctx, suggested, chore.service.Info.Version)
	if err != nil {
		return err
	}

	if !chore.version.IsOutdated {
		return nil
	}

	var notification notifications.NewNotification
	now := chore.nowFn()
	switch {
	case chore.version.FirstTimeSpotted.Add(time.Hour*336).Before(now) && chore.version.TimesNotified == notifications.TimesNotifiedSecond:
		notification = NewVersionNotification(notifications.TimesNotifiedSecond, suggested, chore.nodeID)
		chore.version.TimesNotified = notifications.TimesNotifiedLast

	case chore.version.FirstTimeSpotted.Add(time.Hour*144).Before(now) && chore.version.TimesNotified == notifications.TimesNotifiedFirst:
		notification = NewVersionNotification(notifications.TimesNotifiedFirst, suggested, chore.nodeID)
		chore.version.TimesNotified = notifications.TimesNotifiedSecond

	case chore.version.FirstTimeSpotted.Add(time.Hour*96).Before(now) && chore.version.TimesNotified == notifications.TimesNotifiedZero:
		notification = NewVersionNotification(notifications.TimesNotifiedZero, suggested, chore.nodeID)
		chore.version.TimesNotified = notifications.TimesNotifiedFirst
	default:
		return nil
	}

	_, err = chore.notifications.Receive(ctx, notification)
	if err != nil {
		chore.log.Sugar().Errorf("Failed to receive notification", err.Error())
	}

	return nil
}

// checkVersion checks to make sure the node version is still relevant and returns the suggested
// version. If the node is running below the minimum version, it logs the error but returns
// a nil error to avoid crashing the node.
func (chore *Chore) checkVersion(ctx context.Context) (version.SemVer, error) {
	latest, err := chore.service.CheckVersion(ctx)
	if err != nil && checker.ErrOutdatedVersion.Has(err) {
		chore.log.Error("outdated software version", zap.Error(err))
		return latest, nil
	}

	return latest, err
}

func (chore *Chore) checkRelevance(ctx context.Context, suggested version.SemVer, current version.SemVer) error {
	if current.Compare(suggested) < 0 {
		cursor, err := chore.service.GetCursor(ctx)
		if err != nil {
			return err
		}

		bytes, err := cursor.MarshalJSON()
		if err != nil {
			return err
		}

		cursorString := string(bytes)
		if cursorString != "" {
			cursorString = cursorString[1 : len(cursorString)-1]
		}

		if cursorString == "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" {
			chore.version.IsOutdated = true

			if chore.version.ExpectedVersion.Compare(suggested) < 0 {
				chore.version.ExpectedVersion = suggested
				chore.version.FirstTimeSpotted = time.Now().UTC()
				chore.version.TimesNotified = notifications.TimesNotifiedZero
			}
		} else {
			chore.version.IsOutdated = false
			chore.version.TimesNotified = notifications.TimesNotifiedZero
			return nil
		}
	}
	return nil
}

// Relevance contains information about software being outdated.
type Relevance struct {
	ExpectedVersion  version.SemVer
	IsOutdated       bool
	FirstTimeSpotted time.Time
	TimesNotified    notifications.TimesNotified
}

func (relevance *Relevance) init(currentVer version.SemVer) {
	relevance.ExpectedVersion = currentVer
	relevance.FirstTimeSpotted = time.Now().UTC()
	relevance.TimesNotified = notifications.TimesNotifiedZero
}

// TestSetNow allows tests to have the Service act as if the current time is whatever
// they want. This avoids races and sleeping, making tests more reliable and efficient.
func (chore *Chore) TestSetNow(now func() time.Time) {
	chore.nowFn = now
}

// TestCheckVersion returns chore.relevance, used for chore tests only.
func (chore *Chore) TestCheckVersion() (relevance Relevance) {
	return chore.version
}

// NewVersionNotification - returns version update required notification.
func NewVersionNotification(timesSent notifications.TimesNotified, suggestedVersion version.SemVer, senderID storj.NodeID) (_ notifications.NewNotification) {
	switch timesSent {
	case notifications.TimesNotifiedZero:
		return notifications.NewNotification{
			SenderID: senderID,
			Type:     notifications.TypeCustom,
			Title:    "Please update your Node to Version " + suggestedVersion.String(),
			Message:  "It's time to update your Node's software, new version is available.",
		}
	case notifications.TimesNotifiedFirst:
		return notifications.NewNotification{
			SenderID: senderID,
			Type:     notifications.TypeCustom,
			Title:    "Please update your Node to Version " + suggestedVersion.String(),
			Message:  "It's time to update your Node's software, you are running outdated version!",
		}
	default:
		return notifications.NewNotification{
			SenderID: senderID,
			Type:     notifications.TypeCustom,
			Title:    "Please update your Node to Version " + suggestedVersion.String(),
			Message:  "Last chance to update your software! Your node is running outdated version!",
		}
	}
}
