// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/tenancy"
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
	mailService   *mailservice.Service
	freezeConfig  console.AccountFreezeConfig
	config        Config

	consoleConfig ConsoleConfig

	nowFn func() time.Time
	Loop  *sync2.Cycle
}

// NewOptOutFreezeChore is a constructor for OptOutFreezeChore.
func NewOptOutFreezeChore(log *zap.Logger, usersDB console.Users, freezeService *console.AccountFreezeService, mailService *mailservice.Service, freezeConfig console.AccountFreezeConfig, config Config, consoleConfig ConsoleConfig) *OptOutFreezeChore {
	return &OptOutFreezeChore{
		log:           log,
		freezeService: freezeService,
		usersDB:       usersDB,
		mailService:   mailService,
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
		chore.attemptProcessOptOutEvents(ctx)
		return nil
	})
}

// attemptOptOutFreeze handles both the pre-freeze reminder and the freeze itself.
// In the [OptOutFreezeDate - OptOutFreezeReminderBefore] period, it sends
// a one-time reminder email to eligible users. On or after OptOutFreezeDate it freezes them.
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
	if freezeStart.IsZero() {
		return
	}

	now := chore.nowFn()
	shouldFreeze := !now.Before(freezeStart)
	// we remind users of this freeze some time before the freeze date.
	shouldRemind := chore.config.EmailsEnabled &&
		chore.config.OptOutFreezeReminderBefore > 0 &&
		!now.Before(freezeStart.Add(-chore.config.OptOutFreezeReminderBefore)) &&
		now.Before(freezeStart)

	if !shouldFreeze && !shouldRemind {
		return
	}

	batchSize := chore.config.OptOutFreezeBatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	days := int(chore.config.OptOutFreezeReminderBefore.Hours() / 24)
	freezeDateStr := freezeStart.Format("January 2, 2006")

	totalSent := 0
	totalFrozen := 0
	totalSkipped := 0
	hasNext := true
	var cursor *uuid.UUID

	// Legacy-pricing user agents keep their old pricing and are opt-in exempt so
	// they must not be frozen.
	var excludedUserAgents [][]byte
	for _, ua := range chore.consoleConfig.LegacyPricingUserAgents {
		excludedUserAgents = append(excludedUserAgents, []byte(ua))
	}

	for hasNext {
		page, err := chore.usersDB.ListUsersToOptOutFreeze(ctx, console.ListUsersToOptOutFreezeOptions{
			TenantID:           chore.consoleConfig.TenantID,
			Limit:              batchSize,
			Cursor:             cursor,
			Cutoff:             chore.consoleConfig.NewPricingEffectiveDate,
			ExcludedUserAgents: excludedUserAgents,
		})
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

			switch {
			case shouldFreeze:
				if err := chore.freezeService.OptOutFreezeUser(ctx, userID); err != nil {
					errorLog("Could not opt-out freeze user", err)
					totalSkipped++
					continue
				}
				chore.log.Info("user opt-out frozen",
					zap.String("process", "opt-out freeze"),
					zap.Stringer("user_id", userID),
				)
				totalFrozen++

				if eErr := chore.sendOptOutEmail(ctx, nil, &userID); eErr != nil {
					errorLog("unable to notify user of event", eErr)
				}

			case shouldRemind:
				settings, sErr := chore.usersDB.GetSettings(ctx, userID)
				if sErr != nil && !errors.Is(sErr, sql.ErrNoRows) {
					errorLog("Could not get user settings for pre-freeze reminder", sErr)
					totalSkipped++
					continue
				}
				// Skip users already reminded, and opted-out users who have made their
				// choice and don't need the reminder (they are still frozen on the freeze date).
				if settings != nil && (settings.NoticeDismissal.OptOutFreezeReminderSent || settings.OptInStatus == console.OptedOut) {
					continue
				}

				user, uErr := chore.usersDB.Get(ctx, userID)
				if uErr != nil {
					errorLog("Could not get user for pre-freeze reminder", uErr)
					totalSkipped++
					continue
				}

				message := &console.OptOutFreezePreReminderEmail{
					FreezeDate:  freezeDateStr,
					Days:        days,
					SignInLink:  chore.consoleConfig.ExternalAddress + "/login",
					SupportLink: chore.consoleConfig.GeneralRequestURL,
				}
				emailCtx := ctx
				if chore.consoleConfig.TenantID != nil {
					emailCtx = tenancy.WithContext(ctx, &tenancy.Context{TenantID: *chore.consoleConfig.TenantID})
				}
				chore.mailService.SendRenderedAsync(emailCtx, []post.Address{{Address: user.Email}}, message)
				totalSent++

				noticeDismissal := console.NoticeDismissal{}
				if settings != nil {
					noticeDismissal = settings.NoticeDismissal
				}
				noticeDismissal.OptOutFreezeReminderSent = true
				if uErr = chore.usersDB.UpsertSettings(ctx, userID, console.UpsertUserSettingsRequest{
					NoticeDismissal: &noticeDismissal,
				}); uErr != nil {
					errorLog("Could not mark pre-freeze reminder as sent", uErr)
				}
			}
		}

		hasNext = page.HasNext
		if len(page.IDs) > 0 {
			last := page.IDs[len(page.IDs)-1]
			cursor = &last
		}
	}

	if shouldFreeze {
		chore.log.Info("opt-out freeze executed",
			zap.Int("total_frozen", totalFrozen),
			zap.Int("total_skipped", totalSkipped),
		)
	} else {
		chore.log.Info("opt-out pre-freeze reminders sent",
			zap.Int("total_sent", totalSent),
			zap.Int("total_skipped", totalSkipped),
		)
	}
}

// attemptProcessOptOutEvents escalates opt-out freeze events that need to be and unfreezes users whose
// opt-in status changed and should be unfrozen.
func (chore *OptOutFreezeChore) attemptProcessOptOutEvents(ctx context.Context) {
	var err error
	defer mon.Task()(&ctx)(&err)

	var cursor *console.FreezeEventsByEventAndUserStatusCursor
	hasNext := true

	totalUnfrozen := 0
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
			chore.log.Error("Could not list opt-out events",
				zap.String("process", "opt-out escalation/unfreeze"),
				zap.Error(optOutFreezeError.Wrap(err)),
			)
			return
		}

		for _, event := range events {
			errorLog := func(message, process string, err error) {
				chore.log.Error(message,
					zap.String("process", process),
					zap.Stringer("user_id", event.UserID),
					zap.Error(optOutFreezeError.Wrap(err)),
				)
			}
			infoLog := func(message, process string) {
				chore.log.Info(message,
					zap.String("process", process),
					zap.Stringer("user_id", event.UserID),
				)
			}

			settings, sErr := chore.usersDB.GetSettings(ctx, event.UserID)
			if sErr != nil && !errors.Is(sErr, sql.ErrNoRows) {
				errorLog("Could not get user settings", "opt-out unfreeze", sErr)
				totalSkipped++
				continue
			}

			// If user is OptedIn or Excluded, they get unfrozen.
			if settings != nil && (settings.OptInStatus == console.OptedIn || settings.OptInStatus == console.Excluded) {
				if err := chore.freezeService.OptOutUnfreezeUser(ctx, event.UserID); err != nil {
					errorLog("Could not opt-out unfreeze user", "opt-out unfreeze", err)
					totalSkipped++
					continue
				}
				infoLog("user opt-out unfrozen", "opt-out unfreeze")
				totalUnfrozen++
				continue
			}

			user, err := chore.usersDB.Get(ctx, event.UserID)
			if err != nil {
				errorLog("Could not get user", "opt-out escalate", err)
				totalSkipped++
				continue
			}

			if user.Status == console.Deleted {
				continue
			}

			shouldEscalate, err := chore.freezeService.ShouldEscalateFreezeEvent(ctx, event, chore.nowFn())
			if err != nil {
				errorLog("Could not check if opt-out freeze should escalate", "opt-out escalate", err)
				totalSkipped++
				continue
			}
			if !shouldEscalate {
				continue
			}

			if err := chore.freezeService.EscalateFreezeEvent(ctx, event.UserID, event); err != nil {
				errorLog("Could not escalate opt-out freeze", "opt-out escalate", err)
				totalSkipped++
				continue
			}
			infoLog("user account marked for deletion", "opt-out escalate")
			totalEscalated++

			// update status for sendOptOutEmail
			user.Status = console.PendingDeletion
			if eErr := chore.sendOptOutEmail(ctx, user, nil); eErr != nil {
				errorLog("unable to notify user of event", "opt-out escalate", eErr)
			}
		}

		hasNext = cursor != nil
	}

	chore.log.Info("opt-out events processed",
		zap.Int("total_unfrozen", totalUnfrozen),
		zap.Int("total_escalated", totalEscalated),
		zap.Int("total_skipped", totalSkipped),
	)
}

// sendOptOutEmail sends opt-out freeze emails given a user or userID. The email copy will either be that
// of the freeze notification or the freeze's escalation if the status of the user is PendingDeletion.
func (chore *OptOutFreezeChore) sendOptOutEmail(ctx context.Context, u *console.User, userID *uuid.UUID) (err error) {
	if !chore.config.EmailsEnabled {
		return nil
	}

	var user *console.User
	if u != nil {
		user = u
	} else if userID != nil {
		user, err = chore.usersDB.Get(ctx, *userID)
		if err != nil {
			return err
		}
	} else {
		return errors.New("no user provided")
	}

	// days = 0 will render the pending deletion copy in the email
	// template
	days := 0
	if user.Status != console.PendingDeletion {
		days = int(chore.freezeConfig.OptOutFreezeGracePeriod.Hours() / 24)
	}
	message := &console.OptOutFreezeNotificationEmail{
		Days:        days,
		SignInLink:  chore.consoleConfig.ExternalAddress + "/login",
		SupportLink: chore.consoleConfig.GeneralRequestURL,
	}

	emailCtx := ctx
	if chore.consoleConfig.TenantID != nil {
		emailCtx = tenancy.WithContext(ctx, &tenancy.Context{TenantID: *chore.consoleConfig.TenantID})
	}
	chore.mailService.SendRenderedAsync(emailCtx, []post.Address{{Address: user.Email}}, message)

	return chore.freezeService.IncrementNotificationsCount(ctx, user.ID, console.OptOutFreeze)
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
