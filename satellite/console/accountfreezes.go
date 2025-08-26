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
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ErrAccountFreeze is the class for errors that occur during operation of the account freeze service.
var ErrAccountFreeze = errs.Class("account freeze service")

// ErrNoFreezeStatus is the error for when a user doesn't have a particular freeze status.
var ErrNoFreezeStatus = errs.New("this freeze event does not exist for this user")

// FreezeEventsByEventAndUserStatusCursor is a cursor for getting freeze events by event and user status.
type FreezeEventsByEventAndUserStatusCursor = dbx.Paged_AccountFreezeEvent_By_User_Status_Not_And_AccountFreezeEvent_Event_Continuation

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
	// GetTrialExpirationFreezesToEscalate is a method that gets free trial expiration freezes that correspond to users
	// that are not pending deletion (have not been escalated).
	GetTrialExpirationFreezesToEscalate(ctx context.Context, limit int, cursor *FreezeEventsByEventAndUserStatusCursor) ([]AccountFreezeEvent, *FreezeEventsByEventAndUserStatusCursor, error)
	// GetEscalatedEventsBefore is used to get a list of freeze events of some types that were escalated
	// before the given time (corresponding users have status=PendingDeletion and status_updated_at before olderThan).
	// NB: This method is specifically used to list events for deletion, so a specific event that is not deleted
	// will continue to be returned.
	GetEscalatedEventsBefore(ctx context.Context, params GetEscalatedEventsBeforeParams) (_ []EventWithUser, err error)
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

// EventWithUser contains a freeze event type and the corresponding user ID.
// Returned by GetEscalatedEventsBefore method.
type EventWithUser struct {
	Type   AccountFreezeEventType
	UserID uuid.UUID
}

// GetEscalatedEventsBeforeParams contains parameters for the
// GetEscalatedEventsBefore method.
type GetEscalatedEventsBeforeParams struct {
	Limit      int
	EventTypes []EventTypeAndTime
}

// EventTypeAndTime contains an event type and a time.
// Used to specify which events to query that were created
// before a certain time.
type EventTypeAndTime struct {
	EventType AccountFreezeEventType
	OlderThan time.Time
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

var (
	zeroLimit  = int64(0)
	zeroLimits = map[LimitKind]*int64{
		RateLimit:       &zeroLimit,
		RateLimitGet:    &zeroLimit,
		RateLimitHead:   &zeroLimit,
		RateLimitPut:    &zeroLimit,
		RateLimitList:   &zeroLimit,
		RateLimitDelete: &zeroLimit,

		BurstLimit:       &zeroLimit,
		BurstLimitGet:    &zeroLimit,
		BurstLimitHead:   &zeroLimit,
		BurstLimitPut:    &zeroLimit,
		BurstLimitList:   &zeroLimit,
		BurstLimitDelete: &zeroLimit,
	}
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
	BillingWarnGracePeriod           time.Duration `help:"How long to wait between a billing warning event and billing freezing an account." default:"360h"`
	BillingFreezeGracePeriod         time.Duration `help:"How long to wait between a billing freeze event and setting pending deletion account status." default:"1440h"`
	TrialExpirationFreezeGracePeriod time.Duration `help:"How long to wait between a trail expiration freeze event and setting pending deletion account status. 0 disables escalation." default:"0" testDefault:"720h" devDefault:"720h"`
	TrialExpirationRateLimits        int64         `help:"Specifies the rate and burst limit for 'head', list' and 'delete' operations when a trial account has expired." default:"20"`
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
	if err != nil {
		// If the error is ErrNoRows, it means the user is not frozen.
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, ErrAccountFreeze.Wrap(err)
	}

	return true, nil
}

// billingFreezeUser is a private implementation function that freezes the user specified by the given ID due to nonpayment of invoices.
// The adminInitiated parameter indicates whether this freeze was initiated by an admin or automatically by the satellite.
func (s *AccountFreezeService) billingFreezeUser(ctx context.Context, userID uuid.UUID, adminInitiated bool) (err error) {
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

		// Track using both the old specific method and the new generic method
		s.tracker.TrackAccountFrozen(userID, user.Email, user.HubspotObjectID)
		s.tracker.TrackGenericFreeze(userID, user.Email, BillingFreeze.String(), adminInitiated, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BillingFreezeUser freezes the user specified by the given ID due to nonpayment of invoices.
// This is an automatically triggered freeze (not admin-initiated).
func (s *AccountFreezeService) BillingFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingFreezeUser(ctx, userID, false)
}

// AdminBillingFreezeUser freezes the user specified by the given ID due to nonpayment of invoices.
// This is an admin-initiated freeze.
func (s *AccountFreezeService) AdminBillingFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingFreezeUser(ctx, userID, true)
}

// billingUnfreezeUser is a private implementation function that reverses the billing freeze placed on the user specified by the given ID.
// The adminInitiated parameter indicates whether this unfreeze was initiated by an admin or automatically by the satellite.
func (s *AccountFreezeService) billingUnfreezeUser(ctx context.Context, userID uuid.UUID, adminInitiated bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		event, err := tx.AccountFreezeEvents().Get(ctx, userID, BillingFreeze)
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
			limitUpdates := limitUpdatesFromLimits(limits)
			err = tx.Projects().UpdateLimitsGeneric(ctx, id, limitUpdates)
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

		// Track using both the old specific method and the new generic method
		s.tracker.TrackAccountUnfrozen(userID, user.Email, user.HubspotObjectID)
		s.tracker.TrackGenericUnfreeze(userID, user.Email, BillingFreeze.String(), adminInitiated, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BillingUnfreezeUser reverses the billing freeze placed on the user specified by the given ID.
// This is an automatically triggered unfreeze (not admin-initiated).
func (s *AccountFreezeService) BillingUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingUnfreezeUser(ctx, userID, false)
}

// AdminBillingUnfreezeUser reverses the billing freeze placed on the user specified by the given ID.
// This is an admin-initiated unfreeze.
func (s *AccountFreezeService) AdminBillingUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingUnfreezeUser(ctx, userID, true)
}

// billingWarnUser is a private implementation function that adds a billing warning event to the freeze events table.
// The adminInitiated parameter indicates whether this warning was initiated by an admin or automatically by the satellite.
func (s *AccountFreezeService) billingWarnUser(ctx context.Context, userID uuid.UUID, adminInitiated bool) (err error) {
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

		if freezes.ViolationFreeze != nil || freezes.BillingFreeze != nil || freezes.LegalFreeze != nil {
			return errs.New("User is already frozen")
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
			return err
		}

		// Track using both the old specific method and the new generic method
		s.tracker.TrackAccountFreezeWarning(userID, user.Email, user.HubspotObjectID)
		s.tracker.TrackGenericFreeze(userID, user.Email, BillingWarning.String(), adminInitiated, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BillingWarnUser adds a billing warning event to the freeze events table.
// This is an automatically triggered warning (not admin-initiated).
func (s *AccountFreezeService) BillingWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingWarnUser(ctx, userID, false)
}

// AdminBillingWarnUser adds a billing warning event to the freeze events table.
// This is an admin-initiated warning.
func (s *AccountFreezeService) AdminBillingWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingWarnUser(ctx, userID, true)
}

// billingUnWarnUser is a private implementation function that reverses the warning placed on the user specified by the given ID.
// The adminInitiated parameter indicates whether this unwarning was initiated by an admin or automatically by the satellite.
func (s *AccountFreezeService) billingUnWarnUser(ctx context.Context, userID uuid.UUID, adminInitiated bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		_, err = tx.AccountFreezeEvents().Get(ctx, userID, BillingWarning)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNoFreezeStatus
			}

			return err
		}

		err = tx.AccountFreezeEvents().DeleteByUserIDAndEvent(ctx, userID, BillingWarning)
		if err != nil {
			return err
		}

		// Track using both the old specific method and the new generic method
		s.tracker.TrackAccountUnwarned(userID, user.Email, user.HubspotObjectID)
		s.tracker.TrackGenericUnfreeze(userID, user.Email, BillingWarning.String(), adminInitiated, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BillingUnWarnUser reverses the warning placed on the user specified by the given ID.
// This is an automatically triggered unwarn (not admin-initiated).
func (s *AccountFreezeService) BillingUnWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingUnWarnUser(ctx, userID, false)
}

// AdminBillingUnWarnUser reverses the warning placed on the user specified by the given ID.
// This is an admin-initiated unwarn.
func (s *AccountFreezeService) AdminBillingUnWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.billingUnWarnUser(ctx, userID, true)
}

// ViolationFreezeUser freezes the user specified by the given ID due to ToS violation.
// This is always an admin-initiated action.
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

		// Track the violation freeze using the generic method - always admin initiated
		s.tracker.TrackGenericFreeze(userID, user.Email, ViolationFreeze.String(), true, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// ViolationUnfreezeUser reverses the violation freeze placed on the user specified by the given ID.
// This is always an admin-initiated action.
func (s *AccountFreezeService) ViolationUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		event, err := tx.AccountFreezeEvents().Get(ctx, userID, ViolationFreeze)
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
			limitUpdates := limitUpdatesFromLimits(limits)
			err = tx.Projects().UpdateLimitsGeneric(ctx, id, limitUpdates)
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

		// Track the violation unfreeze using the generic method - always admin initiated
		s.tracker.TrackGenericUnfreeze(userID, user.Email, ViolationFreeze.String(), true, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// LegalFreezeUser freezes the user specified by the given ID for legal review.
// This is always an admin-initiated action.
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
			user:                user,
			newFreezeEvent:      freezes.LegalFreeze,
			existingFreezeEvent: event,
			limits:              limits,
			eventType:           LegalFreeze,
			projectRateLimits:   zeroLimits,
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

		// Track the legal freeze using the generic method - always admin initiated
		s.tracker.TrackGenericFreeze(userID, user.Email, LegalFreeze.String(), true, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// LegalUnfreezeUser reverses the legal freeze placed on the user specified by the given ID.
// This is always an admin-initiated action.
func (s *AccountFreezeService) LegalUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		event, err := tx.AccountFreezeEvents().Get(ctx, userID, LegalFreeze)
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
			limitUpdates := limitUpdatesFromLimits(limits)
			err = tx.Projects().UpdateLimitsGeneric(ctx, id, limitUpdates)
			if err != nil {
				return err
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

		// Track the legal unfreeze using the generic method - always admin initiated
		s.tracker.TrackGenericUnfreeze(userID, user.Email, LegalFreeze.String(), true, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// DelayedBotFreezeUser sets the user specified by the given ID for bot review with some delay.
// This is automatically triggered by the satellite's bot detection, if enabled.
func (s *AccountFreezeService) DelayedBotFreezeUser(ctx context.Context, userID uuid.UUID, days *int) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.store.Users().Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	_, err = s.store.AccountFreezeEvents().Upsert(ctx, &AccountFreezeEvent{
		UserID:             userID,
		Type:               DelayedBotFreeze,
		DaysTillEscalation: days,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	// Track the delayed bot freeze using the generic method - not admin initiated
	s.tracker.TrackGenericFreeze(userID, user.Email, DelayedBotFreeze.String(), false, user.HubspotObjectID)

	return nil
}

// BotFreezeUser freezes the user specified by the given ID for bot review.
// This is an automatically triggered action, not admin-initiated.
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
			user:                user,
			existingFreezeEvent: event,
			limits:              limits,
			eventType:           BotFreeze,
			projectRateLimits:   zeroLimits,
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

		// Track the bot freeze using the generic method
		s.tracker.TrackGenericFreeze(userID, user.Email, BotFreeze.String(), false, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// BotUnfreezeUser reverses the bot freeze placed on the user specified by the given ID.
// This is an automatically triggered action, not admin-initiated.
func (s *AccountFreezeService) BotUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

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
			limitUpdates := limitUpdatesFromLimits(limits)
			err = tx.Projects().UpdateLimitsGeneric(ctx, id, limitUpdates)
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

		// Track the bot unfreeze using the generic method (this is always admin-initiated)
		s.tracker.TrackGenericUnfreeze(userID, user.Email, BotFreeze.String(), true, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// trialExpirationFreezeUser is a private implementation function that freezes the user specified by the given ID due to expired free trial.
// The adminInitiated parameter indicates whether this freeze was initiated by an admin or automatically by the satellite.
func (s *AccountFreezeService) trialExpirationFreezeUser(ctx context.Context, userID uuid.UUID, adminInitiated bool) (err error) {
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

		data := upsertData{
			user:              user,
			newFreezeEvent:    freezes.TrialExpirationFreeze,
			eventType:         TrialExpirationFreeze,
			projectRateLimits: make(map[LimitKind]*int64, len(zeroLimits)),
		}

		data.projectRateLimits[RateLimit] = &zeroLimit
		data.projectRateLimits[RateLimitGet] = &zeroLimit
		data.projectRateLimits[RateLimitPut] = &zeroLimit
		data.projectRateLimits[RateLimitHead] = &s.config.TrialExpirationRateLimits
		data.projectRateLimits[RateLimitList] = &s.config.TrialExpirationRateLimits
		data.projectRateLimits[RateLimitDelete] = &s.config.TrialExpirationRateLimits
		data.projectRateLimits[BurstLimit] = &zeroLimit
		data.projectRateLimits[BurstLimitGet] = &zeroLimit
		data.projectRateLimits[BurstLimitPut] = &zeroLimit
		data.projectRateLimits[BurstLimitHead] = &s.config.TrialExpirationRateLimits
		data.projectRateLimits[BurstLimitList] = &s.config.TrialExpirationRateLimits
		data.projectRateLimits[BurstLimitDelete] = &s.config.TrialExpirationRateLimits

		if s.config.TrialExpirationFreezeGracePeriod != 0 {
			days := int(s.config.TrialExpirationFreezeGracePeriod.Hours() / 24)
			data.daysTillEscalation = &days
		}
		err = s.upsertFreezeEvent(ctx, tx, &data)
		if err != nil {
			return err
		}

		// Track the trial expiration freeze using the generic method
		s.tracker.TrackGenericFreeze(userID, user.Email, TrialExpirationFreeze.String(), adminInitiated, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// TrialExpirationFreezeUser freezes the user specified by the given ID due to expired free trial.
// This is an automatically triggered freeze (not admin-initiated).
func (s *AccountFreezeService) TrialExpirationFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.trialExpirationFreezeUser(ctx, userID, false)
}

// AdminTrialExpirationFreezeUser freezes the user specified by the given ID due to expired free trial.
// This is an admin-initiated freeze.
func (s *AccountFreezeService) AdminTrialExpirationFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.trialExpirationFreezeUser(ctx, userID, true)
}

// trialExpirationUnfreezeUser is a private implementation function that reverses the trial expiration freeze placed on the user specified by the given ID.
// It potentially upgrades a user, setting new limits.
// The adminInitiated parameter indicates whether this unfreeze was initiated by an admin or automatically by the satellite.
func (s *AccountFreezeService) trialExpirationUnfreezeUser(ctx context.Context, userID uuid.UUID, adminInitiated bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		user, err := tx.Users().Get(ctx, userID)
		if err != nil {
			return err
		}

		event, err := tx.AccountFreezeEvents().Get(ctx, userID, TrialExpirationFreeze)
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
			limitUpdates := limitUpdatesFromLimits(limits)
			err = tx.Projects().UpdateLimitsGeneric(ctx, id, limitUpdates)
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

		if user.Status == PendingDeletion {
			status := Active
			err = tx.Users().Update(ctx, userID, UpdateUserRequest{
				Status: &status,
			})
			if err != nil {
				return err
			}
		}

		// Track the trial expiration unfreeze using the generic method
		s.tracker.TrackGenericUnfreeze(userID, user.Email, TrialExpirationFreeze.String(), adminInitiated, user.HubspotObjectID)

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// TrialExpirationUnfreezeUser reverses the trial expiration freeze placed on the user specified by the given ID.
// It potentially upgrades a user, setting new limits.
// This is an automatically triggered unfreeze (not admin-initiated).
func (s *AccountFreezeService) TrialExpirationUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.trialExpirationUnfreezeUser(ctx, userID, false)
}

// AdminTrialExpirationUnfreezeUser reverses the trial expiration freeze placed on the user specified by the given ID.
// It potentially upgrades a user, setting new limits.
// This is an admin-initiated unfreeze.
func (s *AccountFreezeService) AdminTrialExpirationUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	return s.trialExpirationUnfreezeUser(ctx, userID, true)
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

// GetTrialExpirationFreezesToEscalate returns trial expiration freezes that need to be escalated.
func (s *AccountFreezeService) GetTrialExpirationFreezesToEscalate(ctx context.Context, limit int, cursor *FreezeEventsByEventAndUserStatusCursor) (events []AccountFreezeEvent, next *FreezeEventsByEventAndUserStatusCursor, err error) {
	defer mon.Task()(&ctx)(&err)

	events, next, err = s.freezeEventsDB.GetTrialExpirationFreezesToEscalate(ctx, limit, cursor)
	if err != nil {
		return nil, nil, ErrAccountFreeze.Wrap(err)
	}

	return events, next, nil
}

// GetDaysTillEscalation returns the number of days until escalation for a freeze event.
func (s *AccountFreezeService) GetDaysTillEscalation(event AccountFreezeEvent, now time.Time) *int {
	daysTillEscalation := event.DaysTillEscalation

	if event.Type == TrialExpirationFreeze {
		if s.config.TrialExpirationFreezeGracePeriod == 0 {
			return nil
		}
		if daysTillEscalation == nil {
			days := int(s.config.TrialExpirationFreezeGracePeriod.Hours() / 24)
			daysTillEscalation = &days
		}
	}

	if daysTillEscalation == nil {
		return nil
	}
	daysElapsed := int(now.Sub(event.CreatedAt).Hours() / 24)
	diff := *daysTillEscalation - daysElapsed
	return &diff
}

// EscalateFreezeEvent deactivates escalation for this freeze event and sets the user status to pending deletion.
func (s *AccountFreezeService) EscalateFreezeEvent(ctx context.Context, userID uuid.UUID, event AccountFreezeEvent) (err error) {
	defer mon.Task()(&ctx)(&err)

	event.DaysTillEscalation = nil

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		// check if event still exists
		_, err = tx.AccountFreezeEvents().Get(ctx, userID, event.Type)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errs.New("freeze event does not exist")
			}

			return err
		}

		_, err := tx.AccountFreezeEvents().Upsert(ctx, &event)
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

		return nil
	})

	return ErrAccountFreeze.Wrap(err)
}

// ShouldEscalateFreezeEvent checks whether an event's escalation period has elapsed.
func (s *AccountFreezeService) ShouldEscalateFreezeEvent(ctx context.Context, event AccountFreezeEvent, now time.Time) (shouldEscalate bool, err error) {
	defer mon.Task()(&ctx)(&err)

	daysTillEscalation := event.DaysTillEscalation

	if event.Type == TrialExpirationFreeze {
		if s.config.TrialExpirationFreezeGracePeriod == 0 {
			return false, nil
		}
		if daysTillEscalation == nil {
			days := int(s.config.TrialExpirationFreezeGracePeriod.Hours() / 24)
			daysTillEscalation = &days
		}
	}

	if daysTillEscalation == nil {
		return false, nil
	}
	daysElapsed := int(now.Sub(event.CreatedAt).Hours() / 24)
	shouldEscalate = daysElapsed > *daysTillEscalation

	if !shouldEscalate || event.Type != TrialExpirationFreeze {
		return shouldEscalate, nil
	}

	projects, err := s.store.Projects().GetActiveByUserID(ctx, event.UserID)
	if err != nil {
		return false, ErrAccountFreeze.Wrap(err)
	}
	memberProjectCount := 0
	for _, project := range projects {
		if project.OwnerID != event.UserID {
			memberProjectCount++
		}
	}
	if memberProjectCount > 0 {
		return false, nil
	}

	return shouldEscalate, nil
}

// GetEscalatedEventsBefore is used to get a list of freeze events of some types that were escalated
// before the given time.
// NB: This method is specifically used to list events for deletion, so a specific event that is not deleted
// will continue to be returned.
func (s *AccountFreezeService) GetEscalatedEventsBefore(ctx context.Context, params GetEscalatedEventsBeforeParams) (_ []EventWithUser, err error) {
	defer mon.Task()(&ctx)(&err)

	events, err := s.freezeEventsDB.GetEscalatedEventsBefore(ctx, params)
	if err != nil {
		return nil, ErrAccountFreeze.Wrap(err)
	}

	return events, nil
}

// TestChangeFreezeTracker changes the freeze tracker service for tests.
func (s *AccountFreezeService) TestChangeFreezeTracker(t analytics.FreezeTracker) {
	s.tracker = t
}

// TestSetTrialExpirationFreezeGracePeriod changes the trial expiration freeze grace period for tests.
func (s *AccountFreezeService) TestSetTrialExpirationFreezeGracePeriod(period time.Duration) {
	s.config.TrialExpirationFreezeGracePeriod = period
}

func limitUpdatesFromLimits(limits UsageLimits) []Limit {
	toInt64Ptr := func(i *int) *int64 {
		if i == nil {
			return nil
		}
		v := int64(*i)
		return &v
	}

	return []Limit{
		{Kind: BandwidthLimit, Value: &limits.Bandwidth},
		{Kind: UserSetBandwidthLimit, Value: limits.UserSetBandwidthLimit},
		{Kind: StorageLimit, Value: &limits.Storage},
		{Kind: UserSetStorageLimit, Value: limits.UserSetStorageLimit},
		{Kind: SegmentLimit, Value: &limits.Segment},
		{Kind: RateLimit, Value: toInt64Ptr(limits.RateLimit)},
		{Kind: RateLimitGet, Value: toInt64Ptr(limits.RateLimitGet)},
		{Kind: RateLimitDelete, Value: toInt64Ptr(limits.RateLimitDelete)},
		{Kind: RateLimitHead, Value: toInt64Ptr(limits.RateLimitHead)},
		{Kind: RateLimitList, Value: toInt64Ptr(limits.RateLimitList)},
		{Kind: RateLimitPut, Value: toInt64Ptr(limits.RateLimitPut)},
		{Kind: BurstLimit, Value: toInt64Ptr(limits.BurstLimit)},
		{Kind: BurstLimitGet, Value: toInt64Ptr(limits.BurstLimitGet)},
		{Kind: BurstLimitHead, Value: toInt64Ptr(limits.BurstLimitHead)},
		{Kind: BurstLimitDelete, Value: toInt64Ptr(limits.BurstLimitDelete)},
		{Kind: BurstLimitPut, Value: toInt64Ptr(limits.BurstLimitPut)},
		{Kind: BurstLimitList, Value: toInt64Ptr(limits.BurstLimitList)},
	}
}

type upsertData struct {
	user                *User
	newFreezeEvent      *AccountFreezeEvent
	existingFreezeEvent *AccountFreezeEvent
	limits              *AccountFreezeEventLimits
	daysTillEscalation  *int
	eventType           AccountFreezeEventType
	projectRateLimits   map[LimitKind]*int64
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

	projects, err := tx.Projects().GetOwnActive(ctx, data.user.ID)
	if err != nil {
		return err
	}
	for _, p := range projects {
		projLimits := UsageLimits{}
		if p.StorageLimit != nil {
			projLimits.Storage = p.StorageLimit.Int64()
		}
		if p.UserSpecifiedStorageLimit != nil {
			value := p.UserSpecifiedStorageLimit.Int64()
			projLimits.UserSetStorageLimit = &value
		}
		if p.BandwidthLimit != nil {
			projLimits.Bandwidth = p.BandwidthLimit.Int64()
		}
		if p.UserSpecifiedBandwidthLimit != nil {
			value := p.UserSpecifiedBandwidthLimit.Int64()
			projLimits.UserSetBandwidthLimit = &value
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
		if p.RateLimitHead != nil && *p.RateLimitHead != 0 {
			projLimits.RateLimitHead = p.RateLimitHead
		}
		if p.RateLimitGet != nil && *p.RateLimitGet != 0 {
			projLimits.RateLimitGet = p.RateLimitGet
		}
		if p.RateLimitList != nil && *p.RateLimitList != 0 {
			projLimits.RateLimitList = p.RateLimitList
		}
		if p.RateLimitPut != nil && *p.RateLimitPut != 0 {
			projLimits.RateLimitPut = p.RateLimitPut
		}
		if p.RateLimitDelete != nil && *p.RateLimitDelete != 0 {
			projLimits.RateLimitDelete = p.RateLimitDelete
		}
		if p.BurstLimit != nil && *p.BurstLimit != 0 {
			projLimits.BurstLimit = p.BurstLimit
		}
		if p.BurstLimitHead != nil && *p.BurstLimitHead != 0 {
			projLimits.BurstLimitHead = p.BurstLimitHead
		}
		if p.BurstLimitGet != nil && *p.BurstLimitGet != 0 {
			projLimits.BurstLimitGet = p.BurstLimitGet
		}
		if p.BurstLimitList != nil && *p.BurstLimitList != 0 {
			projLimits.BurstLimitList = p.BurstLimitList
		}
		if p.BurstLimitPut != nil && *p.BurstLimitPut != 0 {
			projLimits.BurstLimitPut = p.BurstLimitPut
		}
		if p.BurstLimitDelete != nil && *p.BurstLimitDelete != 0 {
			projLimits.BurstLimitDelete = p.BurstLimitDelete
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
		limits := []Limit{
			{Kind: StorageLimit, Value: &zeroLimit},
			{Kind: UserSetStorageLimit, Value: nil},
			{Kind: BandwidthLimit, Value: &zeroLimit},
			{Kind: UserSetBandwidthLimit, Value: nil},
			{Kind: SegmentLimit, Value: &zeroLimit},
		}

		if len(data.projectRateLimits) > 0 {
			for kind, value := range data.projectRateLimits {
				limits = append(limits, Limit{Kind: kind, Value: value})
			}
		}

		err = tx.Projects().UpdateLimitsGeneric(ctx, proj.ID, limits)
		if err != nil {
			return err
		}
	}

	return nil
}

// IncrementNotificationsCount is a method for incrementing the notification count for a user's account freeze event.
func (s *AccountFreezeService) IncrementNotificationsCount(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) error {
	return ErrAccountFreeze.Wrap(s.freezeEventsDB.IncrementNotificationsCount(ctx, userID, eventType))
}
