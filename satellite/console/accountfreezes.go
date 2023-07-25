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
	GetAll(ctx context.Context, userID uuid.UUID) (freeze *AccountFreezeEvent, warning *AccountFreezeEvent, err error)
	// DeleteAllByUserID is a method for deleting all account freeze events from the database by user ID.
	DeleteAllByUserID(ctx context.Context, userID uuid.UUID) error
	// DeleteByUserIDAndEvent is a method for deleting all account `eventType` events from the database by user ID.
	DeleteByUserIDAndEvent(ctx context.Context, userID uuid.UUID, eventType AccountFreezeEventType) error
}

// AccountFreezeEvent represents an event related to account freezing.
type AccountFreezeEvent struct {
	UserID    uuid.UUID
	Type      AccountFreezeEventType
	Limits    *AccountFreezeEventLimits
	CreatedAt time.Time
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

// AccountFreezeEventType is used to indicate the account freeze event's type.
type AccountFreezeEventType int

const (
	// Freeze signifies that the user has been frozen.
	Freeze AccountFreezeEventType = 0
	// Warning signifies that the user has been warned that they may be frozen soon.
	Warning AccountFreezeEventType = 1
)

// AccountFreezeService encapsulates operations concerning account freezes.
type AccountFreezeService struct {
	freezeEventsDB AccountFreezeEvents
	usersDB        Users
	projectsDB     Projects
	tracker        analytics.FreezeTracker
}

// NewAccountFreezeService creates a new account freeze service.
func NewAccountFreezeService(freezeEventsDB AccountFreezeEvents, usersDB Users, projectsDB Projects, tracker analytics.FreezeTracker) *AccountFreezeService {
	return &AccountFreezeService{
		freezeEventsDB: freezeEventsDB,
		usersDB:        usersDB,
		projectsDB:     projectsDB,
		tracker:        tracker,
	}
}

// IsUserFrozen returns whether the user specified by the given ID is frozen.
func (s *AccountFreezeService) IsUserFrozen(ctx context.Context, userID uuid.UUID) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.freezeEventsDB.Get(ctx, userID, Freeze)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, ErrAccountFreeze.Wrap(err)
	default:
		return true, nil
	}
}

// FreezeUser freezes the user specified by the given ID.
func (s *AccountFreezeService) FreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	freeze, warning, err := s.freezeEventsDB.GetAll(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}
	if warning != nil {
		err = s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, Warning)
		if err != nil {
			return ErrAccountFreeze.Wrap(err)
		}
	}
	if freeze == nil {
		freeze = &AccountFreezeEvent{
			UserID: userID,
			Type:   Freeze,
			Limits: &AccountFreezeEventLimits{
				User: UsageLimits{
					Storage:   user.ProjectStorageLimit,
					Bandwidth: user.ProjectBandwidthLimit,
					Segment:   user.ProjectSegmentLimit,
				},
				Projects: make(map[uuid.UUID]UsageLimits),
			},
		}
	}

	userLimits := UsageLimits{
		Storage:   user.ProjectStorageLimit,
		Bandwidth: user.ProjectBandwidthLimit,
		Segment:   user.ProjectSegmentLimit,
	}
	// If user limits have been zeroed already, we should not override what is in the freeze table.
	if userLimits != (UsageLimits{}) {
		freeze.Limits.User = userLimits
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
			freeze.Limits.Projects[p.ID] = projLimits
		}
	}

	_, err = s.freezeEventsDB.Upsert(ctx, freeze)
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

	s.tracker.TrackAccountFrozen(userID, user.Email)
	return nil
}

// UnfreezeUser reverses the freeze placed on the user specified by the given ID.
func (s *AccountFreezeService) UnfreezeUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	event, err := s.freezeEventsDB.Get(ctx, userID, Freeze)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountFreeze.New("user is not frozen")
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

	err = ErrAccountFreeze.Wrap(s.freezeEventsDB.DeleteAllByUserID(ctx, userID))
	if err != nil {
		return err
	}

	s.tracker.TrackAccountUnfrozen(userID, user.Email)
	return nil
}

// WarnUser adds a warning event to the freeze events table.
func (s *AccountFreezeService) WarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	_, err = s.freezeEventsDB.Upsert(ctx, &AccountFreezeEvent{
		UserID: userID,
		Type:   Warning,
	})
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	s.tracker.TrackAccountFreezeWarning(userID, user.Email)
	return nil
}

// UnWarnUser reverses the warning placed on the user specified by the given ID.
func (s *AccountFreezeService) UnWarnUser(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.usersDB.Get(ctx, userID)
	if err != nil {
		return ErrAccountFreeze.Wrap(err)
	}

	_, err = s.freezeEventsDB.Get(ctx, userID, Warning)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountFreeze.New("user is not warned")
	}

	err = ErrAccountFreeze.Wrap(s.freezeEventsDB.DeleteByUserIDAndEvent(ctx, userID, Warning))
	if err != nil {
		return err
	}

	s.tracker.TrackAccountUnwarned(userID, user.Email)
	return nil
}

// GetAll returns all events for a user.
func (s *AccountFreezeService) GetAll(ctx context.Context, userID uuid.UUID) (freeze *AccountFreezeEvent, warning *AccountFreezeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	freeze, warning, err = s.freezeEventsDB.GetAll(ctx, userID)
	if err != nil {
		return nil, nil, ErrAccountFreeze.Wrap(err)
	}

	return freeze, warning, nil
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

// TestChangeFreezeTracker changes the freeze tracker service for tests.
func (s *AccountFreezeService) TestChangeFreezeTracker(t analytics.FreezeTracker) {
	s.tracker = t
}
