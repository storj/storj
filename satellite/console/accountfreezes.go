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

// ErrFreezeUserStatusUpdate is error returned if updating the user status as part of violation (un)freeze
// fails.
var ErrFreezeUserStatusUpdate = errs.New("user status update failed")

// AccountFreezeEvents exposes methods to manage the account freeze events table in database.
//
// architecture: Database
type AccountFreezeEvents interface {
	// Upsert is a method for updating an account freeze event if it exists and inserting it otherwise.
	Upsert(ctx context.Context, event *AccountFreezeEvent) (*AccountFreezeEvent, error)
	// Get is a method for querying account freeze event from the database by user ID and event type.
	Get(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) (*AccountFreezeEvent, error)
	// GetAllEvents is a method for querying all account freeze events from the database.
	GetAllEvents(ctx context.Context, cursor FreezeEventsCursor) (events *FreezeEventsPage, err error)
	// GetAll is a method for querying all account freeze events from the database by user ID.
	GetAll(ctx context.Context, userID uuid.UUID) (freezes *UserFreezeEvents, err error)
	// DeleteAllByUserID is a method for deleting all account freeze events from the database by user ID.
	DeleteAllByUserID(ctx context.Context, userID uuid.UUID) error
	// DeleteByUserIDAndEvent is a method for deleting all account `eventType` events from the database by user ID.
	DeleteByUserIDAndEvent(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) error
}

// AccountFreezeEvent represents an event related to account freezing.
type AccountFreezeEvent struct {
	UserID             uuid.UUID
	Type               AccountFreezeEventType
	Limits             *AccountFreezeEventLimits
	DaysTillEscalation *int
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
	BillingFreeze, BillingWarning, ViolationFreeze, LegalFreeze *AccountFreezeEvent
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
	freezeEventsDB AccountFreezeEvents
	usersDB        Users
	projectsDB     Projects
	tracker        analytics.FreezeTracker
	config         AccountFreezeConfig
}

// NewAccountFreezeService creates a new account freeze service.
func NewAccountFreezeService(freezeEventsDB AccountFreezeEvents, usersDB Users, projectsDB Projects, tracker analytics.FreezeTracker, config AccountFreezeConfig) *AccountFreezeService {
	return &AccountFreezeService{
		freezeEventsDB: freezeEventsDB,
		usersDB:        usersDB,
		projectsDB:     projectsDB,
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

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	freezes, err := s.freezeEventsDB.GetAll(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}
	if freezes.ViolationFreeze != nil {
		return ErrAccountFreeze.New("User is already frozen due to ToS violation")
	}
	if freezes.LegalFreeze != nil {
		return ErrAccountFreeze.New("User is already frozen for legal review")
	}

	userLimits := UsageLimits{
		Storage:   user.ProjectStorageLimit,
		Bandwidth: user.ProjectBandwidthLimit,
		Segment:   user.ProjectSegmentLimit,
	}

	daysTillEscalation := int(s.config.BillingFreezeGracePeriod.Hours() / 24)
	billingFreeze := freezes.BillingFreeze
	if billingFreeze == nil {
		billingFreeze = &AccountFreezeEvent{
			UserID:             userID,
			Type:               BillingFreeze,
			DaysTillEscalation: &daysTillEscalation,
			Limits: &AccountFreezeEventLimits{
				User:     userLimits,
				Projects: make(map[uuid.UUID]UsageLimits),
			},
		}
	}

	// If user limits have been zeroed already, we should not override what is in the freeze table.
	if userLimits != (UsageLimits{}) {
		billingFreeze.Limits.User = userLimits
	}

	projects, err := s.projectsDB.GetOwn(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
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
		if projLimits != (UsageLimits{}) {
			billingFreeze.Limits.Projects[p.ID] = projLimits
		}
	}

	_, err = s.freezeEventsDB.Upsert(ctx, billingFreeze)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	err = s.usersDB.UpdateUserProjectLimits(ctx, userID, UsageLimits{})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	for _, proj := range projects {
		err := s.projectsDB.UpdateUsageLimits(ctx, proj.ID, UsageLimits{})
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	if freezes.BillingWarning != nil {
		err = s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, BillingWarning)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	s.tracker.TrackAccountFrozen(userID, user.Email)
	return nil
}

// BillingUnfreezeUser reverses the billing freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) BillingUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	event, err := s.freezeEventsDB.Get(ctx, userID, BillingFreeze)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountFreeze.New("user is not frozen due to nonpayment of invoices")
	}

	if event.Limits == nil {
		return ErrAccountFreeze.New("freeze event limits are nil")
	}

	for id, limits := range event.Limits.Projects {
		err := s.projectsDB.UpdateUsageLimits(ctx, id, limits)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	err = s.usersDB.UpdateUserProjectLimits(ctx, userID, event.Limits.User)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	err = ErrAccountFreeze.Wrap(s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, BillingFreeze))
	if err != nil {
		return err
	}

	if user.Status == PendingDeletion {
		status := Active
		err = s.usersDB.Update(ctx, userID, UpdateUserRequest{
			Status: &status,
		})
		if err != nil {
			return ErrAccountFreeze.Wrap(errs.Combine(ErrFreezeUserStatusUpdate, err))
		}
	}

	s.tracker.TrackAccountUnfrozen(userID, user.Email)
	return nil
}

// BillingWarnUser adds a billing warning event to the freeze events table.
func (s *AccountFreezeService) BillingWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	freezes, err := s.freezeEventsDB.GetAll(ctx, userID)
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
	_, err = s.freezeEventsDB.Upsert(ctx, &AccountFreezeEvent{
		UserID:             userID,
		Type:               BillingWarning,
		DaysTillEscalation: &daysTillEscalation,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	s.tracker.TrackAccountFreezeWarning(userID, user.Email)
	return nil
}

// BillingUnWarnUser reverses the warning placed on the user specified by the given ID.
func (s *AccountFreezeService) BillingUnWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	_, err = s.freezeEventsDB.Get(ctx, userID, BillingWarning)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountFreeze.New("user is not warned")
	}

	err = ErrAccountFreeze.Wrap(s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, BillingWarning))
	if err != nil {
		return err
	}

	s.tracker.TrackAccountUnwarned(userID, user.Email)
	return nil
}

// ViolationFreezeUser freezes the user specified by the given ID due to ToS violation.
func (s *AccountFreezeService) ViolationFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	freezes, err := s.freezeEventsDB.GetAll(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	if freezes.LegalFreeze != nil {
		return ErrAccountFreeze.New("User is already frozen for legal review")
	}

	var limits *AccountFreezeEventLimits
	if freezes.BillingFreeze != nil {
		limits = freezes.BillingFreeze.Limits
	}

	userLimits := UsageLimits{
		Storage:   user.ProjectStorageLimit,
		Bandwidth: user.ProjectBandwidthLimit,
		Segment:   user.ProjectSegmentLimit,
	}

	violationFreeze := freezes.ViolationFreeze
	if violationFreeze == nil {
		if limits == nil {
			limits = &AccountFreezeEventLimits{
				User:     userLimits,
				Projects: make(map[uuid.UUID]UsageLimits),
			}
		}
		violationFreeze = &AccountFreezeEvent{
			UserID: userID,
			Type:   ViolationFreeze,
			Limits: limits,
		}
	}

	// If user limits have been zeroed already, we should not override what is in the freeze table.
	if userLimits != (UsageLimits{}) {
		violationFreeze.Limits.User = userLimits
	}

	projects, err := s.projectsDB.GetOwn(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
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
		if projLimits != (UsageLimits{}) {
			violationFreeze.Limits.Projects[p.ID] = projLimits
		}
	}

	_, err = s.freezeEventsDB.Upsert(ctx, violationFreeze)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	err = s.usersDB.UpdateUserProjectLimits(ctx, userID, UsageLimits{})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	for _, proj := range projects {
		err := s.projectsDB.UpdateUsageLimits(ctx, proj.ID, UsageLimits{})
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	status := PendingDeletion
	err = s.usersDB.Update(ctx, userID, UpdateUserRequest{
		Status: &status,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(errs.Combine(ErrFreezeUserStatusUpdate, err))
	}

	if freezes.BillingWarning != nil {
		err = s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, BillingWarning)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	if freezes.BillingFreeze != nil {
		err = s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, BillingFreeze)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	return nil
}

// ViolationUnfreezeUser reverses the violation freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) ViolationUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	event, err := s.freezeEventsDB.Get(ctx, userID, ViolationFreeze)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountFreeze.New("user is not violation frozen")
	}

	if event.Limits == nil {
		return ErrAccountFreeze.New("freeze event limits are nil")
	}

	for id, limits := range event.Limits.Projects {
		err := s.projectsDB.UpdateUsageLimits(ctx, id, limits)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	err = s.usersDB.UpdateUserProjectLimits(ctx, userID, event.Limits.User)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	err = s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, ViolationFreeze)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	status := Active
	err = s.usersDB.Update(ctx, userID, UpdateUserRequest{
		Status: &status,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(errs.Combine(ErrFreezeUserStatusUpdate, err))
	}

	return nil
}

// LegalFreezeUser freezes the user specified by the given ID for legal review.
func (s *AccountFreezeService) LegalFreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	freezes, err := s.freezeEventsDB.GetAll(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}
	if freezes.ViolationFreeze != nil {
		return ErrAccountFreeze.New("User is already frozen due to ToS violation")
	}

	userLimits := UsageLimits{
		Storage:   user.ProjectStorageLimit,
		Bandwidth: user.ProjectBandwidthLimit,
		Segment:   user.ProjectSegmentLimit,
	}

	legalFreeze := freezes.LegalFreeze
	if legalFreeze == nil {
		legalFreeze = &AccountFreezeEvent{
			UserID: userID,
			Type:   LegalFreeze,
			Limits: &AccountFreezeEventLimits{
				User:     userLimits,
				Projects: make(map[uuid.UUID]UsageLimits),
			},
		}
	}

	// If user limits have been zeroed already, we should not override what is in the freeze table.
	if userLimits != (UsageLimits{}) {
		legalFreeze.Limits.User = userLimits
	}

	projects, err := s.projectsDB.GetOwn(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
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
		if projLimits != (UsageLimits{}) {
			legalFreeze.Limits.Projects[p.ID] = projLimits
		}
	}

	_, err = s.freezeEventsDB.Upsert(ctx, legalFreeze)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	err = s.usersDB.UpdateUserProjectLimits(ctx, userID, UsageLimits{})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	for _, proj := range projects {
		err := s.projectsDB.UpdateUsageLimits(ctx, proj.ID, UsageLimits{})
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	if freezes.BillingWarning != nil {
		err = s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, BillingWarning)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	status := LegalHold
	err = s.usersDB.Update(ctx, userID, UpdateUserRequest{
		Status: &status,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(errs.Combine(ErrFreezeUserStatusUpdate, err))
	}

	return nil
}

// LegalUnfreezeUser reverses the legal freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) LegalUnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	event, err := s.freezeEventsDB.Get(ctx, userID, LegalFreeze)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountFreeze.New("user is not legal-frozen")
	}

	if event.Limits == nil {
		return ErrAccountFreeze.New("freeze event limits are nil")
	}

	for id, limits := range event.Limits.Projects {
		err = s.projectsDB.UpdateUsageLimits(ctx, id, limits)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}

	err = s.usersDB.UpdateUserProjectLimits(ctx, userID, event.Limits.User)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	err = ErrAccountFreeze.Wrap(s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, LegalFreeze))
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	if user.Status == LegalHold {
		status := Active
		err = s.usersDB.Update(ctx, userID, UpdateUserRequest{
			Status: &status,
		})
		if err != nil {
			return ErrAccountFreeze.Wrap(errs.Combine(ErrFreezeUserStatusUpdate, err))
		}
	}

	return nil
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

	events, err = s.freezeEventsDB.GetAllEvents(ctx, cursor)
	if err != nil {
		return nil, ErrAccountFreeze.Wrap(err)
	}

	return events, nil
}

// EscalateBillingFreeze deactivates escalation for this freeze event and sets the user status to pending deletion.
func (s *AccountFreezeService) EscalateBillingFreeze(ctx context.Context, userID uuid.UUID, event AccountFreezeEvent) error {
	event.DaysTillEscalation = nil
	_, err := s.freezeEventsDB.Upsert(ctx, &event)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	status := PendingDeletion
	err = s.usersDB.Update(ctx, userID, UpdateUserRequest{
		Status: &status,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	return nil
}

// TestChangeFreezeTracker changes the freeze tracker service for tests.
func (s *AccountFreezeService) TestChangeFreezeTracker(t analytics.FreezeTracker) {
	s.tracker = t
}
