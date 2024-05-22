// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/analytics"
)

// ErrAccountFreeze is the class for errors that occur during operation of the account freeze service.
var ErrAccountFreeze = errs.Class("account freeze service")

// ErrNoFreezeStatus is the error for when a user doesn't have a particular freeze status.
var ErrNoFreezeStatus = errs.New("this freeze event does not exist for this user")

// AccountFreezeEvents exposes methods to manage the account freeze events table in database.
//
// architecture: Database
type AccountFreezeEvents interface {
	// Upsert is a method for updating an account freeze event if it exists and inserting it otherwise.
	Upsert(ctx context.Context, event *AccountFreezeEvent) (*AccountFreezeEvent, error)
	// Get is a method for querying account freeze event from the database by user ID and event type.
	Get(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) (*AccountFreezeEvent, error)
	// GetAllEvents is a method for querying all account freeze events from the database.
	GetAllEvents(ctx context.Context, cursor FreezeEventsCursor, optionalEventTypes []AccountFreezeEventType) (events *FreezeEventsPage, err error)
	// GetAll is a method for querying all account freeze events from the database by user ID.
	GetAll(ctx context.Context, userID uuid.UUID) (freezes *UserFreezeEvents, err error)
	// DeleteAllByUserID is a method for deleting all account freeze events from the database by user ID.
	DeleteAllByUserID(ctx context.Context, userID uuid.UUID) error
	// DeleteByUserIDAndEvent is a method for deleting all account `eventType` events from the database by user ID.
	DeleteByUserIDAndEvent(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) error
	// IncrementNotificationsCount is a method for incrementing the notification count for a user's account freeze event.
	IncrementNotificationsCount(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) error
}

// AccountFreezeEvent represents an event related to account freezing.
type AccountFreezeEvent struct {
	UserID             uuid.UUID
	Type               AccountFreezeEventType
	Limits             *AccountFreezeEventLimits
	DaysTillEscalation *int
	NotificationsCount int
	CreatedAt          time.Time
}

// AccountFreezeEventLimits represents the usage limits for a user's account and projects before they were frozen.
type AccountFreezeEventLimits struct {
	User     UsageLimits               `json:"user"`
	Projects map[uuid.UUID]UsageLimits `json:"projects"`
}

// FreezeEventsCursor holds info for freeze events
// cursor pagination.
type FreezeEventsCursor struct {
	Limit int

	// StartingAfter is the last user ID of the previous page.
	// The next page will start after this user ID.
	StartingAfter *uuid.UUID
}

// FreezeEventsPage returns paginated freeze events.
type FreezeEventsPage struct {
	Events []AccountFreezeEvent
	// Next indicates whether there are more events to retrieve.
	Next bool
}

// UserFreezeEvents holds the freeze events for a user.
type UserFreezeEvents struct {
	BillingFreeze, BillingWarning, ViolationFreeze, LegalFreeze, DelayedBotFreeze, BotFreeze, TrialExpirationFreeze *AccountFreezeEvent
}

// AccountFreezeEventType is used to indicate the account freeze event's type.
type AccountFreezeEventType int

const (
	// BillingFreeze signifies that the user has been frozen due to nonpayment of invoices.
	BillingFreeze AccountFreezeEventType = 0
	// BillingWarning signifies that the user has been warned that they may be frozen soon
	// due to nonpayment of invoices.
	BillingWarning AccountFreezeEventType = 1
	// ViolationFreeze signifies that the user has been frozen due to ToS violation.
	ViolationFreeze AccountFreezeEventType = 2
	// LegalFreeze signifies that the user has been frozen for legal review.
	LegalFreeze AccountFreezeEventType = 3
	// DelayedBotFreeze signifies that the user has to be set to be verified by an admin after some delay.
	DelayedBotFreeze AccountFreezeEventType = 4
	// BotFreeze signifies that the user has been set to be verified by an admin.
	BotFreeze AccountFreezeEventType = 5
	// TrialExpirationFreeze signifies that the user has been frozen because their free trial has expired.
	TrialExpirationFreeze AccountFreezeEventType = 6
)

// String returns a string representation of this event.
func (et AccountFreezeEventType) String() string {
	switch et {
	case BillingFreeze:
		return "Billing Freeze"
	case BillingWarning:
		return "Billing Warning"
	case ViolationFreeze:
		return "Violation Freeze"
	case LegalFreeze:
		return "Legal Freeze"
	case DelayedBotFreeze:
		return "Delayed Bot Freeze"
	case BotFreeze:
		return "Bot Freeze"
	case TrialExpirationFreeze:
		return "Trial Expiration Freeze"
	default:
		return ""
	}
}

// AccountFreezeConfig contains configurable values for account freeze service.
type AccountFreezeConfig struct {
	BillingWarnGracePeriod   time.Duration `help:"How long to wait between a billing warning event and billing freezing an account." default:"360h"`
	BillingFreezeGracePeriod time.Duration `help:"How long to wait between a billing freeze event and setting pending deletion account status." default:"1440h"`
}

// AccountFreezeService encapsulates operations concerning account freezes.
type AccountFreezeService struct {
	store          DB
	freezeEventsDB AccountFreezeEvents
	tracker        analytics.FreezeTracker
	config         AccountFreezeConfig
}

// NewAccountFreezeService creates a new account freeze service.
func NewAccountFreezeService(db DB, tracker analytics.FreezeTracker, config AccountFreezeConfig) *AccountFreezeService {
	return &AccountFreezeService{
		store:          db,
		freezeEventsDB: db.AccountFreezeEvents(),
		tracker:        tracker,
		config:         config,
	}
}

// IsUserBillingFrozen returns whether the user specified by the given ID is frozen
// due to nonpayment of invoices.
func (s *AccountFreezeService) IsUserBillingFrozen(ctx context.Context, userID uuid.UUID) (_ bool, err error) {
	return s.IsUserFrozen(ctx, userID, BillingFreeze)
}

// IsUserViolationFrozen returns whether the user specified by the given ID is frozen.
func (s *AccountFreezeService) IsUserViolationFrozen(ctx context.Context, userID uuid.UUID) (_ bool, err error) {
	return s.IsUserFrozen(ctx, userID, ViolationFreeze)
}

// IsUserFrozen returns whether the user specified by the given ID has an eventType freeze.
func (s *AccountFreezeService) IsUserFrozen(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.freezeEventsDB.Get(ctx, userID, eventType)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, ErrAccountFreeze.Wrap(err)
	default:
		return true, nil
	}
}

// BillingFreezeUser freezes the user specified by the given ID due to nonpayment of invoices.
func (s *AccountFreezeService) BillingFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		freezes, err := tx.AccountFreezeEvents().GetAll(ctx, userID)
		if err != nil {
			return err
		}
		if freezes.ViolationFreeze != nil {
			return errs.New("User is already frozen due to ToS violation")
		}
		if freezes.LegalFreeze != nil {
			return errs.New("User is already frozen for legal review")
		}
		if freezes.DelayedBotFreeze != nil {
			return errs.New("User is already set to be frozen for bot review")
		}
		if freezes.BotFreeze != nil {
			return errs.New("User is already frozen for bot review")
		}

		daysTillEscalation := int(s.config.BillingFreezeGracePeriod.Hours() / 24)
		err = s.upsertFreezeEvent(ctx, tx, &upsertData{
			user:               user,
			newFreezeEvent:     freezes.BillingFreeze,
			daysTillEscalation: &daysTillEscalation,
			eventType:          BillingFreeze,
		})
		if err != nil {
			return err
		}

		if freezes.BillingWarning != nil {
			err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, BillingWarning)
			if err != nil {
				return err
			}
		}
		s.tracker.TrackAccountFrozen(userID, user.Email)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BillingUnfreezeUser reverses the billing freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) BillingUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		event, err := tx.AccountFreezeEvents().Get(ctx, userID, BillingFreeze)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoFreezeStatus
		}

		if event.Limits == nil {
			return errs.New("freeze event limits are nil")
		}

		for id, limits := range event.Limits.Projects {
			err := tx.Projects().UpdateUsageLimits(ctx, id, limits)
			if err != nil {
				return err
			}
		}

		err = tx.Users().UpdateUserProjectLimits(ctx, userID, event.Limits.User)
		if err != nil {
			return err
		}

		err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, BillingFreeze)
		if err != nil {
			return err
		}

		if user.Status == PendingDeletion {
			status := Active
			err = tx.Users().Update(ctx, userID, UpdateUserRequest{
				Status: &status,
			})
			if err != nil {
				return err
			}
		}

		s.tracker.TrackAccountUnfrozen(userID, user.Email)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BillingWarnUser adds a billing warning event to the freeze events table.
func (s *AccountFreezeService) BillingWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		freezes, err := tx.AccountFreezeEvents().GetAll(ctx, userID)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}

		if freezes.ViolationFreeze != nil || freezes.BillingFreeze != nil || freezes.LegalFreeze != nil {
			return ErrAccountFreeze.New("User is already frozen")
		}

		if freezes.BillingWarning != nil {
			return nil
		}

		daysTillEscalation := int(s.config.BillingWarnGracePeriod.Hours() / 24)
		_, err = tx.AccountFreezeEvents().Upsert(ctx, &AccountFreezeEvent{
			UserID:             userID,
			Type:               BillingWarning,
			DaysTillEscalation: &daysTillEscalation,
		})
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}

		s.tracker.TrackAccountFreezeWarning(userID, user.Email)

		return nil
	})

	return err
}

// BillingUnWarnUser reverses the warning placed on the user specified by the given ID.
func (s *AccountFreezeService) BillingUnWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		_, err = tx.AccountFreezeEvents().Get(ctx, userID, BillingWarning)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAccountFreeze.Wrap(errs.Combine(err, ErrNoFreezeStatus))
		}

		err = ErrAccountFreeze.Wrap(tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, BillingWarning))
		if err != nil {
			return err
		}

		s.tracker.TrackAccountUnwarned(userID, user.Email)

		return nil
	})

	return err
}

// ViolationFreezeUser freezes the user specified by the given ID due to ToS violation.
func (s *AccountFreezeService) ViolationFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		freezes, err := tx.AccountFreezeEvents().GetAll(ctx, userID)
		if err != nil {
			return err
		}

		if freezes.LegalFreeze != nil {
			return errs.New("User is already frozen for legal review")
		}
		if freezes.BotFreeze != nil {
			return errs.New("User is already frozen for bot review")
		}

		var limits *AccountFreezeEventLimits
		var event *AccountFreezeEvent
		if freezes.BillingFreeze != nil {
			event = freezes.BillingFreeze
			limits = freezes.BillingFreeze.Limits
		} else if freezes.TrialExpirationFreeze != nil {
			event = freezes.TrialExpirationFreeze
			limits = freezes.TrialExpirationFreeze.Limits
		} else if freezes.BillingWarning != nil {
			event = freezes.BillingWarning
		}

		err = s.upsertFreezeEvent(ctx, tx, &upsertData{
			user:                user,
			newFreezeEvent:      freezes.ViolationFreeze,
			existingFreezeEvent: event,
			limits:              limits,
			eventType:           ViolationFreeze,
		})
		if err != nil {
			return err
		}

		status := PendingDeletion
		err = tx.Users().Update(ctx, userID, UpdateUserRequest{
			Status: &status,
		})
		if err != nil {
			return err
		}

		if event != nil {
			err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, event.Type)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// ViolationUnfreezeUser reverses the violation freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) ViolationUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		event, err := tx.AccountFreezeEvents().Get(ctx, userID, ViolationFreeze)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoFreezeStatus
		}

		if event.Limits == nil {
			return errs.New("freeze event limits are nil")
		}

		for id, limits := range event.Limits.Projects {
			err := tx.Projects().UpdateUsageLimits(ctx, id, limits)
			if err != nil {
				return err
			}
		}

		err = tx.Users().UpdateUserProjectLimits(ctx, userID, event.Limits.User)
		if err != nil {
			return err
		}

		err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, ViolationFreeze)
		if err != nil {
			return err
		}

		status := Active
		err = tx.Users().Update(ctx, userID, UpdateUserRequest{
			Status: &status,
		})
		if err != nil {
			return err
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// LegalFreezeUser freezes the user specified by the given ID for legal review.
func (s *AccountFreezeService) LegalFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		freezes, err := tx.AccountFreezeEvents().GetAll(ctx, userID)
		if err != nil {
			return err
		}
		if freezes.ViolationFreeze != nil {
			return errs.New("User is already frozen due to ToS violation")
		}
		if freezes.BotFreeze != nil {
			return errs.New("User is already frozen for bot review")
		}

		var limits *AccountFreezeEventLimits
		var event *AccountFreezeEvent
		if freezes.BillingFreeze != nil {
			event = freezes.BillingFreeze
			limits = event.Limits
		} else if freezes.TrialExpirationFreeze != nil {
			event = freezes.TrialExpirationFreeze
			limits = event.Limits
		} else if freezes.BillingWarning != nil {
			event = freezes.BillingWarning
		}

		err = s.upsertFreezeEvent(ctx, tx, &upsertData{
			user:                 user,
			newFreezeEvent:       freezes.LegalFreeze,
			existingFreezeEvent:  event,
			limits:               limits,
			eventType:            LegalFreeze,
			zeroProjectRateLimit: true,
		})
		if err != nil {
			return err
		}

		if event != nil {
			err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, event.Type)
			if err != nil {
				return err
			}
		}

		status := LegalHold
		err = tx.Users().Update(ctx, userID, UpdateUserRequest{
			Status: &status,
		})
		if err != nil {
			return err
		}

		// Invalidate all user sessions.
		_, err = tx.WebappSessions().DeleteAllByUserID(ctx, user.ID)
		if err != nil {
			return err
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// LegalUnfreezeUser reverses the legal freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) LegalUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		event, err := tx.AccountFreezeEvents().Get(ctx, userID, LegalFreeze)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoFreezeStatus
		}

		if event.Limits == nil {
			return errs.New("freeze event limits are nil")
		}

		for id, limits := range event.Limits.Projects {
			err = tx.Projects().UpdateUsageLimits(ctx, id, limits)
			if err != nil {
				return err
			}

			// remove rate limit
			err = tx.Projects().UpdateRateLimit(ctx, id, limits.RateLimit)
			if err != nil {
				return ErrAccountFreeze.Wrap(err)
			}
			// remove burst limit
			err = tx.Projects().UpdateBurstLimit(ctx, id, limits.BurstLimit)
			if err != nil {
				return ErrAccountFreeze.Wrap(err)
			}
		}

		err = tx.Users().UpdateUserProjectLimits(ctx, userID, event.Limits.User)
		if err != nil {
			return err
		}

		err = ErrAccountFreeze.Wrap(tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, LegalFreeze))
		if err != nil {
			return err
		}

		if user.Status == LegalHold {
			status := Active
			err = tx.Users().Update(ctx, userID, UpdateUserRequest{
				Status: &status,
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// DelayedBotFreezeUser sets the user specified by the given ID for bot review with some delay.
func (s *AccountFreezeService) DelayedBotFreezeUser(ctx context.Context, userID uuid.UUID, days *int) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.store.AccountFreezeEvents().Upsert(ctx, &AccountFreezeEvent{
		UserID:             userID,
		Type:               DelayedBotFreeze,
		DaysTillEscalation: days,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	return nil
}

// BotFreezeUser freezes the user specified by the given ID for bot review.
func (s *AccountFreezeService) BotFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		if user.Status == PendingBotVerification {
			return errs.New("user is frozen pending bot verification")
		}

		freezes, err := tx.AccountFreezeEvents().GetAll(ctx, userID)
		if err != nil {
			return err
		}
		if freezes.BotFreeze != nil {
			return errs.New("User is already frozen for bot review")
		}

		var limits *AccountFreezeEventLimits
		var event *AccountFreezeEvent
		if freezes.BillingFreeze != nil {
			event = freezes.BillingFreeze
			limits = event.Limits
		} else if freezes.TrialExpirationFreeze != nil {
			event = freezes.TrialExpirationFreeze
			limits = event.Limits
		}

		err = s.upsertFreezeEvent(ctx, tx, &upsertData{
			user:                 user,
			existingFreezeEvent:  event,
			limits:               limits,
			eventType:            BotFreeze,
			zeroProjectRateLimit: true,
		})
		if err != nil {
			return err
		}

		for _, freezeType := range []AccountFreezeEventType{DelayedBotFreeze, BillingFreeze, TrialExpirationFreeze} {
			err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, freezeType)
			if err != nil {
				return err
			}
		}

		botStatus := PendingBotVerification
		err = tx.Users().Update(ctx, userID, UpdateUserRequest{Status: &botStatus})
		if err != nil {
			return err
		}

		// Invalidate all user sessions.
		_, err = tx.WebappSessions().DeleteAllByUserID(ctx, user.ID)
		if err != nil {
			return err
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BotUnfreezeUser reverses the bot freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) BotUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		event, err := tx.AccountFreezeEvents().Get(ctx, userID, BotFreeze)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNoFreezeStatus
			}
			return err
		}

		if event.Limits == nil {
			return errs.New("freeze event limits are nil")
		}

		for id, limits := range event.Limits.Projects {
			err = tx.Projects().UpdateUsageLimits(ctx, id, limits)
			if err != nil {
				return err
			}

			// remove rate limit
			err = tx.Projects().UpdateRateLimit(ctx, id, limits.RateLimit)
			if err != nil {
				return err
			}
			// remove burst limit
			err = tx.Projects().UpdateBurstLimit(ctx, id, limits.BurstLimit)
			if err != nil {
				return err
			}
		}

		err = tx.Users().UpdateUserProjectLimits(ctx, userID, event.Limits.User)
		if err != nil {
			return err
		}

		err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, BotFreeze)
		if err != nil {
			return err
		}

		activeStatus := Active
		err = tx.Users().Update(ctx, userID, UpdateUserRequest{Status: &activeStatus})
		if err != nil {
			return err
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// TrialExpirationFreezeUser freezes the user specified by the given ID due to expired free trial.
func (s *AccountFreezeService) TrialExpirationFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		freezes, err := tx.AccountFreezeEvents().GetAll(ctx, userID)
		if err != nil {
			return err
		}
		if freezes.ViolationFreeze != nil {
			return errs.New("User is already frozen due to ToS violation")
		}
		if freezes.LegalFreeze != nil {
			return errs.New("User is already frozen for legal review")
		}
		if freezes.BotFreeze != nil {
			return errs.New("User is already frozen for bot review")
		}

		err = s.upsertFreezeEvent(ctx, tx, &upsertData{
			user:                 user,
			newFreezeEvent:       freezes.TrialExpirationFreeze,
			eventType:            TrialExpirationFreeze,
			zeroProjectRateLimit: true,
		})
		if err != nil {
			return err
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// TrialExpirationUnfreezeUser reverses the trial expiration freeze placed on the user specified by the given ID.
// It potentially upgrades a user, setting new limits.
func (s *AccountFreezeService) TrialExpirationUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		_, err = tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		event, err := tx.AccountFreezeEvents().Get(ctx, userID, TrialExpirationFreeze)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoFreezeStatus
		}

		if event.Limits == nil {
			return errs.New("freeze event limits are nil")
		}

		for id, limits := range event.Limits.Projects {
			err := tx.Projects().UpdateUsageLimits(ctx, id, limits)
			if err != nil {
				return err
			}

			// remove rate limit
			err = tx.Projects().UpdateRateLimit(ctx, id, limits.RateLimit)
			if err != nil {
				return err
			}
			// remove burst limit
			err = tx.Projects().UpdateBurstLimit(ctx, id, limits.BurstLimit)
			if err != nil {
				return err
			}
		}

		err = tx.Users().UpdateUserProjectLimits(ctx, userID, event.Limits.User)
		if err != nil {
			return err
		}

		err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, TrialExpirationFreeze)
		if err != nil {
			return err
		}

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// Get returns an event of a specific type for a user.
func (s *AccountFreezeService) Get(ctx context.Context, userID uuid.UUID, freezeType AccountFreezeEventType) (event *AccountFreezeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	event, err = s.freezeEventsDB.Get(ctx, userID, freezeType)
	if err != nil {
		return nil, ErrAccountFreeze.Wrap(err)
	}

	return event, nil
}

// GetAll returns all events for a user.
func (s *AccountFreezeService) GetAll(ctx context.Context, userID uuid.UUID) (freezes *UserFreezeEvents, err error) {
	defer mon.Task()(&ctx)(&err)

	freezes, err = s.freezeEventsDB.GetAll(ctx, userID)
	if err != nil {
		return nil, ErrAccountFreeze.Wrap(err)
	}

	return freezes, nil
}

// GetAllEvents returns all events.
func (s *AccountFreezeService) GetAllEvents(ctx context.Context, cursor FreezeEventsCursor) (events *FreezeEventsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	events, err = s.freezeEventsDB.GetAllEvents(ctx, cursor, nil)
	if err != nil {
		return nil, ErrAccountFreeze.Wrap(err)
	}

	return events, nil
}

// GetAllEventsByType returns all events by event type.
func (s *AccountFreezeService) GetAllEventsByType(ctx context.Context, cursor FreezeEventsCursor, eventTypes []AccountFreezeEventType) (events *FreezeEventsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	events, err = s.freezeEventsDB.GetAllEvents(ctx, cursor, eventTypes)
	if err != nil {
		return nil, ErrAccountFreeze.Wrap(err)
	}

	return events, nil
}

// EscalateBillingFreeze deactivates escalation for this freeze event and sets the user status to pending deletion.
func (s *AccountFreezeService) EscalateBillingFreeze(ctx context.Context, userID uuid.UUID, event AccountFreezeEvent) (err error) {
	defer mon.Task()(&ctx)(&err)

	event.DaysTillEscalation = nil

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		_, err := tx.AccountFreezeEvents().Upsert(ctx, &event)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}

		status := PendingDeletion
		err = tx.Users().Update(ctx, userID, UpdateUserRequest{
			Status: &status,
		})
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}

		return nil
	})

	return err
}

// TestChangeFreezeTracker changes the freeze tracker service for tests.
func (s *AccountFreezeService) TestChangeFreezeTracker(t analytics.FreezeTracker) {
	s.tracker = t
}

type upsertData struct {
	user                 *User
	newFreezeEvent       *AccountFreezeEvent
	existingFreezeEvent  *AccountFreezeEvent
	limits               *AccountFreezeEventLimits
	daysTillEscalation   *int
	eventType            AccountFreezeEventType
	zeroProjectRateLimit bool
}

func (s *AccountFreezeService) upsertFreezeEvent(ctx context.Context, tx DBTx, data *upsertData) error {
	userLimits := UsageLimits{
		Storage:   data.user.ProjectStorageLimit,
		Bandwidth: data.user.ProjectBandwidthLimit,
		Segment:   data.user.ProjectSegmentLimit,
	}

	if data.newFreezeEvent == nil {
		if data.limits == nil {
			data.limits = &AccountFreezeEventLimits{
				User:     userLimits,
				Projects: make(map[uuid.UUID]UsageLimits),
			}
		}

		data.newFreezeEvent = &AccountFreezeEvent{
			UserID:             data.user.ID,
			Type:               data.eventType,
			DaysTillEscalation: data.daysTillEscalation,
			Limits:             data.limits,
		}
	}

	// If user limits have been zeroed already, we should not override what is in the freeze table.
	if userLimits != (UsageLimits{}) {
		data.newFreezeEvent.Limits.User = userLimits
	}

	projects, err := tx.Projects().GetOwn(ctx, data.user.ID)
	if err != nil {
		return err
	}
	for _, p := range projects {
		projLimits := UsageLimits{}
		if p.StorageLimit != nil {
			projLimits.Storage = p.StorageLimit.Int64()
		}
		if p.BandwidthLimit != nil {
			projLimits.Bandwidth = p.BandwidthLimit.Int64()
		}
		if p.SegmentLimit != nil {
			projLimits.Segment = *p.SegmentLimit
		}
		// If project limits have been zeroed already, we should not override what is in the freeze table.
		if projLimits == (UsageLimits{}) {
			if data.existingFreezeEvent == nil || data.existingFreezeEvent.Limits == nil {
				continue
			}
			// if limits were zeroed in a billing freeze, we should use those
			projLimits = data.existingFreezeEvent.Limits.Projects[p.ID]
		}

		if p.RateLimit != nil && *p.RateLimit != 0 {
			projLimits.RateLimit = p.RateLimit
		}
		if p.BurstLimit != nil && *p.BurstLimit != 0 {
			projLimits.BurstLimit = p.BurstLimit
		}

		data.newFreezeEvent.Limits.Projects[p.ID] = projLimits
	}

	_, err = tx.AccountFreezeEvents().Upsert(ctx, data.newFreezeEvent)
	if err != nil {
		return err
	}

	err = tx.Users().UpdateUserProjectLimits(ctx, data.user.ID, UsageLimits{})
	if err != nil {
		return err
	}

	for _, proj := range projects {
		err := tx.Projects().UpdateUsageLimits(ctx, proj.ID, UsageLimits{})
		if err != nil {
			return err
		}

		if data.zeroProjectRateLimit {
			// zero project's rate limit to prevent lists/deletes
			zeroLimit := 0
			err = tx.Projects().UpdateRateLimit(ctx, proj.ID, &zeroLimit)
			if err != nil {
				return err
			}

			err = tx.Projects().UpdateBurstLimit(ctx, proj.ID, &zeroLimit)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// IncrementNotificationsCount is a method for incrementing the notification count for a user's account freeze event.
func (s *AccountFreezeService) IncrementNotificationsCount(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) error {
	return Error.Wrap(s.freezeEventsDB.IncrementNotificationsCount(ctx, userID, eventType))
}
