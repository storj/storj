// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

var optOutFreezeError = errs.Class("opt-out-freeze-chore")

// OptOutFreezeChore is a chore that drives the opt-out freeze flow: on or after the configured
// OptOutFreezeDate, batches of users whose OptInStatus is not OptedIn (and not Excluded) get
// OptOutFreeze events; OptOutFreeze events past their grace period get escalated to
// PendingDeletion.
type OptOutFreezeChore struct {
	log           *zap.Logger
	freezeService *console.AccountFreezeService
	usersDB       console.Users
	freezeConfig  console.AccountFreezeConfig
	config        Config

	consoleConfig ConsoleConfig

	nowFn func() time.Time
	Loop  *sync2.Cycle
}

// NewOptOutFreezeChore is a constructor for OptOutFreezeChore.
func NewOptOutFreezeChore(log *zap.Logger, usersDB console.Users, freezeService *console.AccountFreezeService, freezeConfig console.AccountFreezeConfig, config Config, consoleConfig ConsoleConfig) *OptOutFreezeChore {
	return &OptOutFreezeChore{
		log:           log,
		freezeService: freezeService,
		usersDB:       usersDB,
		freezeConfig:  freezeConfig,
		config:        config,
		consoleConfig: consoleConfig,
		nowFn:         time.Now,
		Loop:          sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *OptOutFreezeChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		chore.attemptOptOutFreeze(ctx)
		chore.attemptEscalateOptOutFreeze(ctx)
		return nil
	})
}

// attemptOptOutFreeze opt-out freezes users who have opted out or have made no action after
// OptOutFreezeDate has passed.
func (chore *OptOutFreezeChore) attemptOptOutFreeze(ctx context.Context) {
	var err error
	defer mon.Task()(&ctx)(&err)

	freezeStart, err := chore.freezeConfig.OptOutFreezeStartTime()
	if err != nil {
		chore.log.Error("Could not parse OptOutFreezeDate",
			zap.String("process", "opt-out freeze"),
			zap.String("value", chore.freezeConfig.OptOutFreezeDate),
			zap.Error(optOutFreezeError.Wrap(err)),
		)
		return
	}
	if freezeStart.IsZero() || chore.nowFn().Before(freezeStart) {
		return
	}

	batchSize := chore.config.OptOutFreezeBatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	totalFrozen := 0
	totalSkipped := 0
	hasNext := true
	var cursor *uuid.UUID

	for hasNext {
		page, err := chore.usersDB.ListUsersToOptOutFreeze(ctx, batchSize, cursor)
		if err != nil {
			chore.log.Error("Could not list users to opt-out freeze",
				zap.String("process", "opt-out freeze"),
				zap.Error(optOutFreezeError.Wrap(err)),
			)
			return
		}

		for _, userID := range page.IDs {
			errorLog := func(message string, err error) {
				chore.log.Error(message,
					zap.String("process", "opt-out freeze"),
					zap.Stringer("user_id", userID),
					zap.Error(optOutFreezeError.Wrap(err)),
				)
			}
			infoLog := func(message string) {
				chore.log.Info(message,
					zap.String("process", "opt-out freeze"),
					zap.Stringer("user_id", userID),
				)
			}

			if err := chore.freezeService.OptOutFreezeUser(ctx, userID); err != nil {
				errorLog("Could not opt-out freeze user", err)
				totalSkipped++
				continue
			}
			infoLog("user opt-out frozen")
			totalFrozen++
		}

		hasNext = page.HasNext
		if len(page.IDs) > 0 {
			last := page.IDs[len(page.IDs)-1]
			cursor = &last
		}
	}

	chore.log.Info("opt-out freeze executed",
		zap.Int("total_frozen", totalFrozen),
		zap.Int("total_skipped", totalSkipped),
	)
}

// attemptEscalateOptOutFreeze escalates OptOutFreeze events past their grace period to PendingDeletion.
func (chore *OptOutFreezeChore) attemptEscalateOptOutFreeze(ctx context.Context) {
	var err error
	defer mon.Task()(&ctx)(&err)

	var cursor *console.FreezeEventsByEventAndUserStatusCursor
	hasNext := true
	totalEscalated := 0
	totalSkipped := 0

	getEvents := func(c *console.FreezeEventsByEventAndUserStatusCursor) (events []console.AccountFreezeEvent, err error) {
		events, cursor, err = chore.freezeService.GetOptOutFreezesToEscalate(ctx, chore.consoleConfig.TenantID, 100, c)
		if err != nil {
			return nil, err
		}
		return events, err
	}

	for hasNext {
		events, err := getEvents(cursor)
		if err != nil {
			chore.log.Error("Could not list opt-out freeze events",
				zap.String("process", "opt-out escalate"),
				zap.Error(optOutFreezeError.Wrap(err)),
			)
			return
		}

		for _, event := range events {
			errorLog := func(message string, err error) {
				chore.log.Error(message,
					zap.String("process", "opt-out escalate"),
					zap.Stringer("user_id", event.UserID),
					zap.Error(optOutFreezeError.Wrap(err)),
				)
			}
			infoLog := func(message string) {
				chore.log.Info(message,
					zap.String("process", "opt-out escalate"),
					zap.Stringer("user_id", event.UserID),
				)
			}

			user, err := chore.usersDB.Get(ctx, event.UserID)
			if err != nil {
				errorLog("Could not get user", err)
				totalSkipped++
				continue
			}

			if user.Status == console.Deleted {
				totalSkipped++
				continue
			}

			shouldEscalate, err := chore.freezeService.ShouldEscalateFreezeEvent(ctx, event, chore.nowFn())
			if err != nil {
				errorLog("Could not check if opt-out freeze should escalate", err)
				totalSkipped++
				continue
			}
			if !shouldEscalate {
				continue
			}

			if err := chore.freezeService.EscalateFreezeEvent(ctx, event.UserID, event); err != nil {
				errorLog("Could not escalate opt-out freeze", err)
				totalSkipped++
				continue
			}
			infoLog("user account marked for deletion")
			totalEscalated++
		}

		hasNext = cursor != nil
	}

	chore.log.Info("opt-out freezes escalated",
		zap.Int("total_marked_for_deletion", totalEscalated),
		zap.Int("total_skipped", totalSkipped),
	)
}

// TestSetNow sets nowFn on chore for testing.
func (chore *OptOutFreezeChore) TestSetNow(f func() time.Time) {
	chore.nowFn = f
}

// TestSetFreezeService changes the freeze service for tests.
func (chore *OptOutFreezeChore) TestSetFreezeService(service *console.AccountFreezeService) {
	chore.freezeService = service
}

// Close closes the chore.
func (chore *OptOutFreezeChore) Close() error {
	chore.Loop.Close()
	return nil
}
