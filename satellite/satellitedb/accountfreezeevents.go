// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/tagsql"
)

// Ensure that accountFreezeEvents implements console.AccountFreezeEvents.
var _ console.AccountFreezeEvents = (*accountFreezeEvents)(nil)

// accountFreezeEvents is an implementation of console.AccountFreezeEvents.
type accountFreezeEvents struct {
	db *satelliteDB
}

// Upsert is a method for updating an account freeze event if it exists and inserting it otherwise.
func (events *accountFreezeEvents) Upsert(ctx context.Context, event *console.AccountFreezeEvent) (_ *console.AccountFreezeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	if event == nil {
		return nil, Error.New("event is nil")
	}

	createFields := dbx.AccountFreezeEvent_Create_Fields{}
	if event.DaysTillEscalation != nil {
		createFields.DaysTillEscalation = dbx.AccountFreezeEvent_DaysTillEscalation(*event.DaysTillEscalation)
	}
	createFields.NotificationsCount = dbx.AccountFreezeEvent_NotificationsCount(event.NotificationsCount)
	if event.Limits != nil {
		limitBytes, err := json.Marshal(event.Limits)
		if err != nil {
			return nil, err
		}
		createFields.Limits = dbx.AccountFreezeEvent_Limits(limitBytes)
	}

	dbxEvent, err := events.db.Replace_AccountFreezeEvent(ctx,
		dbx.AccountFreezeEvent_UserId(event.UserID.Bytes()),
		dbx.AccountFreezeEvent_Event(int(event.Type)),
		createFields,
	)
	if err != nil {
		return nil, err
	}

	return fromDBXAccountFreezeEvent(dbxEvent)
}

// Get is a method for querying account freeze event from the database by user ID and event type.
func (events *accountFreezeEvents) Get(ctx context.Context, userID uuid.UUID, eventType console.AccountFreezeEventType) (event *console.AccountFreezeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxEvent, err := events.db.Get_AccountFreezeEvent_By_UserId_And_Event(ctx,
		dbx.AccountFreezeEvent_UserId(userID.Bytes()),
		dbx.AccountFreezeEvent_Event(int(eventType)),
	)
	if err != nil {
		return nil, err
	}

	return fromDBXAccountFreezeEvent(dbxEvent)
}

// GetAllEvents is a method for querying all account freeze events or events of particular types from the database.
func (events *accountFreezeEvents) GetAllEvents(ctx context.Context, cursor console.FreezeEventsCursor, optionalEventTypes []console.AccountFreezeEventType) (freezeEvents *console.FreezeEventsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	if cursor.Limit <= 0 {
		return nil, errs.New("limit cannot be zero or less")
	}

	page := console.FreezeEventsPage{
		Events: make([]console.AccountFreezeEvent, 0, cursor.Limit),
	}

	if cursor.StartingAfter == nil {
		cursor.StartingAfter = &uuid.UUID{}
	}

	var rows tagsql.Rows
	if len(optionalEventTypes) == 0 {
		rows, err = events.db.Query(ctx, events.db.Rebind(`
		SELECT user_id, event, days_till_escalation, notifications_count, created_at
		FROM account_freeze_events
			WHERE user_id > ?
			ORDER BY user_id LIMIT ?
		`), cursor.StartingAfter, cursor.Limit+1)
	} else {
		types := make([]string, 0, len(optionalEventTypes))
		for _, t := range optionalEventTypes {
			types = append(types, strconv.Itoa(int(t)))
		}
		rows, err = events.db.Query(ctx, events.db.Rebind(`
		SELECT user_id, event, days_till_escalation, notifications_count, created_at
		FROM account_freeze_events
			WHERE user_id > ? AND event IN (`+strings.Join(types, ",")+`)
			ORDER BY user_id LIMIT ?
		`), cursor.StartingAfter, cursor.Limit+1)
	}

	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	count := 0
	for rows.Next() {
		count++
		if count > cursor.Limit {
			// we are done with this page; do not include this event
			page.Next = true
			break
		}
		var event dbx.AccountFreezeEvent
		err = rows.Scan(&event.UserId, &event.Event, &event.DaysTillEscalation, &event.NotificationsCount, &event.CreatedAt)
		if err != nil {
			return nil, err
		}

		eventToSend, err := fromDBXAccountFreezeEvent(&event)
		if err != nil {
			return nil, err
		}

		page.Events = append(page.Events, *eventToSend)
	}

	return &page, rows.Err()
}

// GetAll is a method for querying all account freeze events from the database by user ID.
func (events *accountFreezeEvents) GetAll(ctx context.Context, userID uuid.UUID) (freezes *console.UserFreezeEvents, err error) {
	defer mon.Task()(&ctx)(&err)

	// dbxEvents will have a max length of 6.
	// because there's at most 1 instance each of 6 types of events for a user.
	dbxEvents, err := events.db.All_AccountFreezeEvent_By_UserId(ctx,
		dbx.AccountFreezeEvent_UserId(userID.Bytes()),
	)
	if err != nil {
		return nil, err
	}

	freezes = &console.UserFreezeEvents{}
	for _, event := range dbxEvents {
		eventType := console.AccountFreezeEventType(event.Event)
		if eventType == console.BillingFreeze {
			freezes.BillingFreeze, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, err
			}
			continue
		}
		if eventType == console.ViolationFreeze {
			freezes.ViolationFreeze, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, err
			}
			continue
		}
		if eventType == console.LegalFreeze {
			freezes.LegalFreeze, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, err
			}
			continue
		}
		if eventType == console.BillingWarning {
			freezes.BillingWarning, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, err
			}
		}
		if eventType == console.DelayedBotFreeze {
			freezes.DelayedBotFreeze, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, err
			}
		}
		if eventType == console.BotFreeze {
			freezes.BotFreeze, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, err
			}
		}
		if eventType == console.TrialExpirationFreeze {
			freezes.TrialExpirationFreeze, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, err
			}
		}
	}

	return freezes, nil
}

// DeleteAllByUserID is a method for deleting all account freeze events from the database by user ID.
func (events *accountFreezeEvents) DeleteAllByUserID(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = events.db.Delete_AccountFreezeEvent_By_UserId(ctx, dbx.AccountFreezeEvent_UserId(userID.Bytes()))

	return err
}

// DeleteByUserIDAndEvent is a method for deleting all account `eventType` events from the database by user ID.
func (events *accountFreezeEvents) DeleteByUserIDAndEvent(ctx context.Context, userID uuid.UUID, eventType console.AccountFreezeEventType) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = events.db.Delete_AccountFreezeEvent_By_UserId_And_Event(ctx,
		dbx.AccountFreezeEvent_UserId(userID.Bytes()),
		dbx.AccountFreezeEvent_Event(int(eventType)),
	)

	return err
}

// fromDBXAccountFreezeEvent converts *dbx.AccountFreezeEvent to *console.AccountFreezeEvent.
func fromDBXAccountFreezeEvent(dbxEvent *dbx.AccountFreezeEvent) (_ *console.AccountFreezeEvent, err error) {
	if dbxEvent == nil {
		return nil, Error.New("dbx event is nil")
	}
	userID, err := uuid.FromBytes(dbxEvent.UserId)
	if err != nil {
		return nil, err
	}
	event := &console.AccountFreezeEvent{
		UserID:             userID,
		Type:               console.AccountFreezeEventType(dbxEvent.Event),
		DaysTillEscalation: dbxEvent.DaysTillEscalation,
		NotificationsCount: dbxEvent.NotificationsCount,
		CreatedAt:          dbxEvent.CreatedAt,
	}
	if dbxEvent.Limits != nil {
		err := json.Unmarshal(dbxEvent.Limits, &event.Limits)
		if err != nil {
			return nil, err
		}
	}
	return event, nil
}
