// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/satellite/console"
)

var botFreezeError = errs.Class("bot-freeze-chore")

// BotFreezeChore is a chore that handles bot freezes.
type BotFreezeChore struct {
	log           *zap.Logger
	freezeService *console.AccountFreezeService
	usersDB       console.Users

	flagBots bool

	nowFn func() time.Time
	Loop  *sync2.Cycle
}

// NewBotFreezeChore is a constructor for BotFreezeChore.
func NewBotFreezeChore(log *zap.Logger, usersDB console.Users, freezeService *console.AccountFreezeService, config Config, flagBots bool) *BotFreezeChore {
	return &BotFreezeChore{
		log:           log,
		freezeService: freezeService,
		usersDB:       usersDB,
		flagBots:      flagBots,
		nowFn:         time.Now,
		Loop:          sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *BotFreezeChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !chore.flagBots {
		chore.log.Info("Bot freeze chore is disabled; skipping run")
		return nil
	}
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		chore.attemptBotFreeze(ctx)
		return nil
	})
}

func (chore *BotFreezeChore) attemptBotFreeze(ctx context.Context) {
	var err error
	defer mon.Task()(&ctx)(&err)

	cursor := console.FreezeEventsCursor{
		Limit: 100,
	}
	hasNext := true
	usersCount := 0
	frozenCount := 0

	getEvents := func(c console.FreezeEventsCursor) (events *console.FreezeEventsPage, err error) {
		events, err = chore.freezeService.GetAllEventsByType(ctx, c, []console.AccountFreezeEventType{console.DelayedBotFreeze})
		if err != nil {
			return nil, err
		}
		return events, err
	}

	for hasNext {
		events, err := getEvents(cursor)
		if err != nil {
			return
		}

		for _, event := range events.Events {
			errorLog := func(message string, err error) {
				chore.log.Error(message,
					zap.String("process", "delayed bot freeze"),
					zap.Any("userID", event.UserID),
					zap.Error(botFreezeError.Wrap(err)),
				)
			}
			infoLog := func(message string) {
				chore.log.Info(message,
					zap.String("process", "delayed bot freeze"),
					zap.Any("userID", event.UserID),
					zap.Error(botFreezeError.Wrap(err)),
				)
			}

			if event.Type != console.DelayedBotFreeze {
				infoLog("Skipping non delayed bot freeze event")
				continue
			}

			user, err := chore.usersDB.Get(ctx, event.UserID)
			if err != nil {
				errorLog("Could not get user", err)
				continue
			}

			if user.Status == console.PendingBotVerification {
				infoLog("Skipping event; account already bot frozen")
				continue
			}

			usersCount++

			shouldEscalate, err := chore.freezeService.ShouldEscalateFreezeEvent(ctx, event, chore.nowFn())
			if err != nil {
				errorLog("Could not check if delayed bot freeze should escalate", err)
				continue
			}
			if !shouldEscalate {
				infoLog("Skipping event; as it shouldn't be escalated yet")
				continue
			}

			err = chore.freezeService.BotFreezeUser(ctx, event.UserID)
			if err != nil {
				errorLog("Could not bot freeze user", err)
				continue
			}

			frozenCount++
		}

		hasNext = events.Next
		if length := len(events.Events); length > 0 {
			cursor.StartingAfter = &events.Events[length-1].UserID
		}
	}

	chore.log.Info("delayed bot freeze executed",
		zap.Int("user total", usersCount),
		zap.Int("total frozen", frozenCount),
	)
}

// TestSetNow sets nowFn on chore for testing.
func (chore *BotFreezeChore) TestSetNow(f func() time.Time) {
	chore.nowFn = f
}

// TestSetFreezeService changes the freeze service for tests.
func (chore *BotFreezeChore) TestSetFreezeService(service *console.AccountFreezeService) {
	chore.freezeService = service
}

// Close closes the chore.
func (chore *BotFreezeChore) Close() error {
	chore.Loop.Close()
	return nil
}
