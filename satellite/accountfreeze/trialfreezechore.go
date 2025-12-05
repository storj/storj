// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
)

var trialFreezeError = errs.Class("trial-freeze-chore")

// TrialFreezeChore is a chore that handles trial expiration freezes.
type TrialFreezeChore struct {
	log           *zap.Logger
	freezeService *console.AccountFreezeService
	usersDB       console.Users
	mailService   *mailservice.Service
	freezeConfig  console.AccountFreezeConfig

	externalAddress   string
	generalRequestURL string

	nowFn func() time.Time
	Loop  *sync2.Cycle
}

// NewTrialFreezeChore is a constructor for TrialFreezeChore.
func NewTrialFreezeChore(log *zap.Logger, usersDB console.Users, freezeService *console.AccountFreezeService, mailService *mailservice.Service, freezeConfig console.AccountFreezeConfig, config Config, externalAddress, generalRequestURL string) *TrialFreezeChore {
	return &TrialFreezeChore{
		log:               log,
		freezeService:     freezeService,
		usersDB:           usersDB,
		freezeConfig:      freezeConfig,
		mailService:       mailService,
		externalAddress:   externalAddress,
		generalRequestURL: generalRequestURL,
		nowFn:             time.Now,
		Loop:              sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *TrialFreezeChore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {

		chore.attemptTrialExpirationFreeze(ctx)

		chore.attemptEscalateTrialExpirationFreeze(ctx)

		return nil
	})
}

func (chore *TrialFreezeChore) attemptTrialExpirationFreeze(ctx context.Context) {
	var err error
	defer mon.Task()(&ctx)(&err)

	limit := 100
	totalFrozen := 0

	for {
		users, err := chore.usersDB.GetExpiredFreeTrialsAfter(ctx, chore.nowFn(), limit)
		if err != nil {
			chore.log.Error("Unable to list expired free trials",
				zap.String("process", "trial expiration freeze"),
				zap.Error(trialFreezeError.Wrap(err)),
			)
			break
		}

		if len(users) == 0 {
			chore.log.Info("No expired free trials found",
				zap.String("process", "trial expiration freeze"),
			)
			break
		}

		for _, user := range users {
			err = chore.freezeService.TrialExpirationFreezeUser(ctx, user.ID)
			if err == nil {
				totalFrozen++
				continue
			}
			chore.log.Error("Could not trial expiration freeze user",
				zap.String("process", "trial expiration freeze"),
				zap.Any("userID", user.ID),
				zap.Error(trialFreezeError.Wrap(err)),
			)
		}
	}

	chore.log.Info("trial expiration freeze executed",
		zap.Int("totalFrozen", totalFrozen))
}

func (chore *TrialFreezeChore) attemptEscalateTrialExpirationFreeze(ctx context.Context) {
	var err error
	defer mon.Task()(&ctx)(&err)

	totalMarkedForDeletion := 0
	totalSkipped := 0

	var cursor *console.FreezeEventsByEventAndUserStatusCursor
	hasNext := true

	getEvents := func(c *console.FreezeEventsByEventAndUserStatusCursor) (events []console.AccountFreezeEvent, err error) {
		events, cursor, err = chore.freezeService.GetTrialExpirationFreezesToEscalate(ctx, 100, c)
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

		for _, event := range events {
			shouldEscalate, err := chore.freezeService.ShouldEscalateFreezeEvent(ctx, event, chore.nowFn())
			if err != nil {
				chore.log.Error("Could not check if trial expiration freeze should escalate",
					zap.String("process", "trial expiration freeze escalation"),
					zap.Any("userID", event.UserID),
					zap.Error(trialFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}
			if !shouldEscalate {
				chore.log.Info("Skipping user; freeze event should not escalate",
					zap.String("process", "trial expiration freeze escalation"),
					zap.Any("userID", event.UserID),
				)
				totalSkipped++
				continue
			}

			user, err := chore.usersDB.Get(ctx, event.UserID)
			if err != nil {
				chore.log.Error("Could not get user for escalation",
					zap.String("process", "trial expiration freeze escalation"),
					zap.Any("userID", event.UserID),
					zap.Error(trialFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			if user.Status != console.Active {
				chore.log.Info("Skipping user; account not active",
					zap.String("process", "trial expiration freeze escalation"),
					zap.Any("userID", event.UserID),
					zap.String("status", user.Status.String()),
				)
				totalSkipped++
				continue
			}

			if user.Kind != console.FreeUser {
				// This accounts for users that were moved from console.FreeUser
				// to console.PaidUser or console.NFRUser after they'd been TrailExpiration frozen
				// and the freeze was not cleared.
				chore.log.Info("Skipping user; user is not in free trial",
					zap.String("process", "trial expiration freeze escalation"),
					zap.Any("userID", event.UserID),
				)
				totalSkipped++

				// clear the freeze event if it exists
				err = chore.freezeService.TrialExpirationUnfreezeUser(ctx, event.UserID)
				if err != nil {
					chore.log.Error("Could not trial expiration unfreeze non-trial user",
						zap.String("process", "trial expiration freeze escalation"),
						zap.Any("userID", event.UserID),
						zap.Error(trialFreezeError.Wrap(err)),
					)
				} else {
					chore.log.Info("Non-trial user unfrozen",
						zap.String("process", "trial expiration freeze escalation"),
						zap.Any("userID", event.UserID),
					)
				}
				continue
			}

			err = chore.freezeService.EscalateFreezeEvent(ctx, event.UserID, event)
			if err != nil {
				chore.log.Error("Could not escalate trial expiration freeze",
					zap.String("process", "trial expiration freeze escalation"),
					zap.Any("userID", event.UserID),
					zap.Error(trialFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			eErr := chore.sendEmail(ctx, user, &event)
			if eErr != nil {
				chore.log.Error("Could not send user email",
					zap.String("process", "trial expiration freeze escalation"),
					zap.Any("userID", event.UserID),
					zap.Error(trialFreezeError.Wrap(eErr)),
				)
			}
			totalMarkedForDeletion++
		}

		hasNext = cursor != nil
	}

	chore.log.Info("trial expiration freezes escalated",
		zap.Int("totalMarkedForDeletion", totalMarkedForDeletion),
		zap.Int("totalSkipped", totalSkipped),
	)
}

func (chore *TrialFreezeChore) sendEmail(ctx context.Context, user *console.User, event *console.AccountFreezeEvent) error {
	signInLink := chore.externalAddress + "/login"
	supportLink := chore.generalRequestURL
	elapsedTime := int(chore.nowFn().Sub(event.CreatedAt).Hours() / 24)

	incrementNotificationCount := true
	var message mailservice.Message
	switch event.Type {
	case console.BillingWarning:
		days := int(chore.freezeConfig.BillingWarnGracePeriod.Hours() / 24)
		if event.NotificationsCount != 0 {
			days = *event.DaysTillEscalation - elapsedTime
		}
		message = &console.BillingWarningEmail{
			EmailNumber: event.NotificationsCount + 1,
			Days:        days,
			SignInLink:  signInLink,
			SupportLink: supportLink,
		}
	case console.BillingFreeze:
		days := int(chore.freezeConfig.BillingFreezeGracePeriod.Hours() / 24)
		if event.NotificationsCount != 0 {
			days = *event.DaysTillEscalation - elapsedTime
		}
		message = &console.BillingFreezeNotificationEmail{
			EmailNumber: event.NotificationsCount + 1,
			Days:        days,
			SignInLink:  signInLink,
			SupportLink: supportLink,
		}
	case console.TrialExpirationFreeze:
		incrementNotificationCount = false
		message = &console.TrialExpirationEscalationReminderEmail{
			SupportLink: supportLink,
		}
	default:
		return trialFreezeError.New("unknown event type")
	}

	chore.mailService.SendRenderedAsync(ctx, []post.Address{{Address: user.Email}}, message)

	if incrementNotificationCount {
		err := chore.freezeService.IncrementNotificationsCount(ctx, user.ID, event.Type)
		if err != nil {
			return err
		}
	}

	return nil
}

// TestSetNow sets nowFn on chore for testing.
func (chore *TrialFreezeChore) TestSetNow(f func() time.Time) {
	chore.nowFn = f
}

// TestSetFreezeService changes the freeze service for tests.
func (chore *TrialFreezeChore) TestSetFreezeService(service *console.AccountFreezeService) {
	chore.freezeService = service
}

// Close closes the chore.
func (chore *TrialFreezeChore) Close() error {
	chore.Loop.Close()
	return nil
}
