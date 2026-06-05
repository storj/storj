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
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/tenancy"
)

var inactivityFreezeError = errs.Class("inactivity-freeze-chore")

// InactivityFreezeChore detects accounts with sustained zero effective usage
// potentially freezes them.
type InactivityFreezeChore struct {
	log           *zap.Logger
	freezeService *console.AccountFreezeService
	usersDB       console.Users
	projectsDB    console.Projects
	projectUsage  accounting.ProjectAccounting
	payments      payments.Accounts
	stripeDB      stripe.DB
	mailService   *mailservice.Service
	freezeConfig  console.AccountFreezeConfig
	config        Config
	consoleConfig ConsoleConfig

	nowFn func() time.Time
	Loop  *sync2.Cycle
}

// NewInactivityFreezeChore creates a new InactivityFreezeChore.
func NewInactivityFreezeChore(
	log *zap.Logger,
	usersDB console.Users,
	projectsDB console.Projects,
	projectUsage accounting.ProjectAccounting,
	payments payments.Accounts,
	stripeDB stripe.DB,
	freezeService *console.AccountFreezeService,
	mailService *mailservice.Service,
	freezeConfig console.AccountFreezeConfig,
	config Config,
	consoleConfig ConsoleConfig,
) *InactivityFreezeChore {
	return &InactivityFreezeChore{
		log:           log,
		freezeService: freezeService,
		usersDB:       usersDB,
		projectsDB:    projectsDB,
		projectUsage:  projectUsage,
		payments:      payments,
		stripeDB:      stripeDB,
		mailService:   mailService,
		freezeConfig:  freezeConfig,
		config:        config,
		consoleConfig: consoleConfig,
		nowFn:         time.Now,
		Loop:          sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *InactivityFreezeChore) Run(ctx context.Context) error {
	defer mon.Task()(&ctx)(nil)
	return chore.Loop.Run(ctx, func(ctx context.Context) (err error) {
		if !chore.config.InactivitySuspendEnabled {
			return nil
		}
		chore.attemptInactivityWarn(ctx)
		chore.attemptInactivityFreezeOrCancel(ctx)
		return nil
	})
}

// attemptInactivityWarn iterates active paid accounts and warns those with a configured consecutive
// zero-revenue months.
func (chore *InactivityFreezeChore) attemptInactivityWarn(ctx context.Context) {
	var err error
	defer mon.Task()(&ctx)(&err)

	now := chore.nowFn()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var (
		batchSize    = 100
		cursor       = (*uuid.UUID)(nil)
		hasNext      = true
		totalWarned  = 0
		totalSkipped = 0
	)

	for hasNext {
		page, err := chore.usersDB.ListUsersForInactivityCheck(ctx, chore.consoleConfig.TenantID, batchSize, cursor)
		if err != nil {
			chore.log.Error("could not list users for inactivity check",
				zap.Error(inactivityFreezeError.Wrap(err)),
			)
			return
		}

		for _, userID := range page.IDs {
			user, err := chore.usersDB.Get(ctx, userID)
			if err != nil {
				chore.log.Error("could not get user",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			settings, err := chore.usersDB.GetSettings(ctx, userID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				chore.log.Error("could not get user settings",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			if settings != nil && settings.InactivityExempt {
				totalSkipped++
				continue
			}

			freezes, err := chore.freezeService.GetAll(ctx, userID)
			if err != nil {
				chore.log.Error("could not get freeze events",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			if chore.mustSkipForInactivity(user, freezes) {
				totalSkipped++
				continue
			}

			windowStart := currentMonthStart.AddDate(0, -chore.config.InactivityConsecutiveZeroCycles, 0)
			allZero, err := chore.isZeroUsageInRange(ctx, userID, windowStart, currentMonthStart)
			if err != nil {
				chore.log.Error("could not check zero usage months",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}
			if !allZero {
				continue
			}

			if err := chore.freezeService.InactivityWarnUser(ctx, userID); err != nil {
				chore.log.Error("could not warn user for inactivity",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}
			totalWarned++

			if chore.config.EmailsEnabled {
				if eErr := chore.sendInactivityEmail(ctx, user, console.InactivityWarning); eErr != nil {
					chore.log.Error("could not send inactivity warning email",
						zap.Stringer("user_id", userID),
						zap.Error(inactivityFreezeError.Wrap(eErr)),
					)
				}
			}
		}

		hasNext = page.HasNext
		if len(page.IDs) > 0 {
			last := page.IDs[len(page.IDs)-1]
			cursor = &last
		}
	}

	chore.log.Info("inactivity warn executed",
		zap.Int("total_warned", totalWarned),
		zap.Int("total_skipped", totalSkipped),
	)
}

// attemptInactivityFreezeOrCancel iterates users in the InactivityWarning state and either
// escalates to freeze or removes the warning (usage detected).
func (chore *InactivityFreezeChore) attemptInactivityFreezeOrCancel(ctx context.Context) {
	defer mon.Task()(&ctx)(nil)

	var (
		now    = chore.nowFn()
		cursor = console.FreezeEventsCursor{
			Limit:    100,
			TenantID: chore.consoleConfig.TenantID,
		}
		hasNext       = true
		totalFrozen   = 0
		totalUnwarned = 0
		totalSkipped  = 0
	)

	for hasNext {
		eventsPage, err := chore.freezeService.GetAllEventsByType(ctx, cursor, []console.AccountFreezeEventType{console.InactivityWarning})
		if err != nil {
			chore.log.Error("could not list inactivity warning events",
				zap.Error(inactivityFreezeError.Wrap(err)),
			)
			return
		}

		for _, event := range eventsPage.Events {
			userID := event.UserID

			user, err := chore.usersDB.Get(ctx, userID)
			if err != nil {
				chore.log.Error("could not get user",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			settings, err := chore.usersDB.GetSettings(ctx, userID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				chore.log.Error("could not get user settings",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			if settings != nil && settings.InactivityExempt {
				totalSkipped++
				continue
			}

			freezes, err := chore.freezeService.GetAll(ctx, userID)
			if err != nil {
				chore.log.Error("could not get freeze events",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			if chore.mustSkipForInactivity(user, freezes) {
				totalSkipped++
				continue
			}

			zero, err := chore.isZeroUsageFromAccounting(ctx, userID, event.CreatedAt, chore.nowFn())
			if err != nil {
				chore.log.Error("could not check usage since warning",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}

			if !zero {
				if err := chore.freezeService.InactivityUnwarnUser(ctx, userID); err != nil {
					chore.log.Error("could not cancel inactivity warning",
						zap.Stringer("user_id", userID),
						zap.Error(inactivityFreezeError.Wrap(err)),
					)
					totalSkipped++
					continue
				}
				totalUnwarned++
				continue
			}

			if now.Before(event.CreatedAt.Add(chore.freezeConfig.InactivityGracePeriod)) {
				continue
			}

			if err := chore.freezeService.InactivityFreezeUser(ctx, userID); err != nil {
				chore.log.Error("could not freeze user for inactivity",
					zap.Stringer("user_id", userID),
					zap.Error(inactivityFreezeError.Wrap(err)),
				)
				totalSkipped++
				continue
			}
			totalFrozen++

			if chore.config.EmailsEnabled {
				if eErr := chore.sendInactivityEmail(ctx, user, console.InactivityFreeze); eErr != nil {
					chore.log.Error("could not send inactivity freeze email",
						zap.Stringer("user_id", userID),
						zap.Error(inactivityFreezeError.Wrap(eErr)),
					)
				}
			}
		}

		hasNext = eventsPage.Next
		if length := len(eventsPage.Events); length > 0 {
			cursor.StartingAfter = &eventsPage.Events[length-1].UserID
		}
	}

	chore.log.Info("inactivity freeze/unwarn executed",
		zap.Int("total_frozen", totalFrozen),
		zap.Int("total_unwarned", totalUnwarned),
		zap.Int("total_skipped", totalSkipped),
	)
}

// mustSkipForInactivity returns true if the user should be excluded from inactivity processing.
func (chore *InactivityFreezeChore) mustSkipForInactivity(user *console.User, freezes *console.UserFreezeEvents) bool {
	if user.IsBillingExempt() {
		return true
	}
	if user.Status != console.Active && user.Status != console.UserRequestedDeletion {
		return true
	}

	// Skip users who haven't been on the paid tier long enough to have completed
	// InactivityConsecutiveZeroCycles.
	now := chore.nowFn()
	checkWindowStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, -chore.config.InactivityConsecutiveZeroCycles, 0)
	if user.UpgradeTime != nil && !user.UpgradeTime.Before(checkWindowStart) {
		return true
	}

	if freezes.BillingFreeze != nil || freezes.ViolationFreeze != nil || freezes.LegalFreeze != nil ||
		freezes.BotFreeze != nil || freezes.TrialExpirationFreeze != nil || freezes.OptOutFreeze != nil {
		return true
	}
	if freezes.InactivityFreeze != nil {
		return true
	}
	return false
}

// isZeroUsageInRange returns true if the user had zero effective revenue in [since, before).
// It makes a single query to either Stripe invoices or accounting records.
func (chore *InactivityFreezeChore) isZeroUsageInRange(ctx context.Context, userID uuid.UUID, since, before time.Time) (bool, error) {
	_, err := chore.stripeDB.Customers().GetCustomerID(ctx, userID)
	if err != nil && !errs.Is(err, stripe.ErrNoCustomer) {
		return false, inactivityFreezeError.Wrap(err)
	}
	if err == nil {
		return chore.isZeroUsageFromInvoices(ctx, userID, since, before)
	}
	return chore.isZeroUsageFromAccounting(ctx, userID, since, before)
}

// isZeroUsageFromAccounting checks accounting records for all projects owned by the user.
func (chore *InactivityFreezeChore) isZeroUsageFromAccounting(ctx context.Context, userID uuid.UUID, since, before time.Time) (bool, error) {
	projects, err := chore.projectsDB.GetOwnActive(ctx, userID)
	if err != nil {
		return false, inactivityFreezeError.Wrap(err)
	}
	priceModel := chore.payments.GetProjectUsagePriceModel()
	for _, project := range projects {
		usage, err := chore.projectUsage.GetProjectTotal(ctx, project.ID, since, before)
		if err != nil {
			return false, inactivityFreezeError.Wrap(err)
		}

		// not real cost estimation; we just want to know if this usage could amount
		// to a zero invoice.
		cost := chore.payments.CalculateProjectUsagePrice(*usage, priceModel)
		if !cost.Storage.IsZero() || !cost.Egress.IsZero() || !cost.Segment.IsZero() {
			return false, nil
		}
	}
	return true, nil
}

// isZeroUsageFromInvoices checks Stripe invoice history for the target month.
// A month is zero-revenue if no invoice exists (case 2) or all invoices have Amount == 0 (case 3).
func (chore *InactivityFreezeChore) isZeroUsageFromInvoices(ctx context.Context, userID uuid.UUID, monthStart, monthEnd time.Time) (bool, error) {
	invoices, err := chore.payments.Invoices().List(ctx, &userID)
	if err != nil {
		return false, inactivityFreezeError.Wrap(err)
	}
	for _, inv := range invoices {
		if !inv.Start.Before(monthStart) && inv.Start.Before(monthEnd) && inv.Amount != 0 {
			return false, nil
		}
	}
	return true, nil
}

// sendInactivityEmail sends the appropriate email for the given event type.
func (chore *InactivityFreezeChore) sendInactivityEmail(ctx context.Context, user *console.User, eventType console.AccountFreezeEventType) error {
	signInLink := chore.consoleConfig.ExternalAddress + "/login"
	supportLink := chore.consoleConfig.GeneralRequestURL

	emailCtx := ctx
	if chore.consoleConfig.TenantID != nil {
		emailCtx = tenancy.WithContext(ctx, &tenancy.Context{TenantID: *chore.consoleConfig.TenantID})
	}

	var message mailservice.Message
	switch eventType {
	case console.InactivityWarning:
		message = &console.InactivityWarningEmail{
			GracePeriodDays: int(chore.freezeConfig.InactivityGracePeriod.Hours() / 24),
			SignInLink:      signInLink,
			SupportLink:     supportLink,
		}
	case console.InactivityFreeze:
		message = &console.InactivityFreezeEmail{
			SignInLink:  signInLink,
			SupportLink: supportLink,
		}
	default:
		return inactivityFreezeError.New("unknown inactivity event type: %v", eventType)
	}

	chore.mailService.SendRenderedAsync(emailCtx, []post.Address{{Address: user.Email}}, message)
	return nil
}

// TestSetNow sets the nowFn for testing.
func (chore *InactivityFreezeChore) TestSetNow(f func() time.Time) {
	chore.nowFn = f
}

// TestSetFreezeService replaces the freeze service for testing.
func (chore *InactivityFreezeChore) TestSetFreezeService(service *console.AccountFreezeService) {
	chore.freezeService = service
}

// Close closes the chore.
func (chore *InactivityFreezeChore) Close() error {
	chore.Loop.Close()
	return nil
}
