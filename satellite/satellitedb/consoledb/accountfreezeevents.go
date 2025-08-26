// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/tagsql"
)

var (
	mon = monkit.Package()

	// Error is the default satellitedb errs class.
	Error = errs.Class("consoledb")
)

// Ensure that accountFreezeEvents implements console.AccountFreezeEvents.
var _ console.AccountFreezeEvents = (*accountFreezeEvents)(nil)

// accountFreezeEvents is an implementation of console.AccountFreezeEvents.
type accountFreezeEvents struct {
	db dbx.DriverMethods
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
		rows, err = events.db.QueryContext(ctx, events.db.Rebind(`
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
		rows, err = events.db.QueryContext(ctx, events.db.Rebind(`
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

// GetTrialExpirationFreezesToEscalate is a method that gets free trial expiration freezes that correspond to users
// that are not pending deletion (have not been escalated).
func (events *accountFreezeEvents) GetTrialExpirationFreezesToEscalate(ctx context.Context, limit int, cursor *console.FreezeEventsByEventAndUserStatusCursor) (_ []console.AccountFreezeEvent, next *console.FreezeEventsByEventAndUserStatusCursor, err error) {
	defer mon.Task()(&ctx)(&err)

	evs, next, err := events.db.Paged_AccountFreezeEvent_By_User_Status_Not_And_AccountFreezeEvent_Event(
		ctx,
		// where user.status != pending_deletion
		dbx.User_Status(int(console.PendingDeletion)),
		// and event = trial_expiration_freeze
		dbx.AccountFreezeEvent_Event(int(console.TrialExpirationFreeze)),
		limit,
		cursor,
	)
	if err != nil {
		return nil, nil, err
	}
	eventsToReturn := make([]console.AccountFreezeEvent, 0, len(evs))
	for _, ev := range evs {
		event, err := fromDBXAccountFreezeEvent(ev)
		if err != nil {
			return nil, nil, err
		}
		eventsToReturn = append(eventsToReturn, *event)
	}
	return eventsToReturn, next, nil
}

// GetEscalatedEventsBefore is used to get a list of freeze events of some types that were escalated
// before the given time.
// NB: This method is specifically used to list events for deletion, so a specific event that is not deleted
// will continue to be returned.
func (events *accountFreezeEvents) GetEscalatedEventsBefore(ctx context.Context, params console.GetEscalatedEventsBeforeParams) (_ []console.EventWithUser, err error) {
	defer mon.Task()(&ctx)(&err)

	baseQuery := `
			SELECT afe.event as event, u.id AS user_id
				FROM account_freeze_events AS afe
			JOIN users AS u 
				ON u.id = afe.user_id
			WHERE u.status = ?
				AND u.status_updated_at < ?
				AND afe.event = ?
			ORDER BY u.status_updated_at ASC`

	query := fmt.Sprintf("%s\nLIMIT ?", baseQuery)

	queryParams := make([]interface{}, 0)
	if len(params.EventTypes) > 1 {
		/*
			craft a query like this:
			SELECT event, user_id, project_id FROM (
				(SELECT ...
					JOIN ...
					WHERE u.status = ?
						AND u.status_updated_at < ?
						AND afe.event = ?
					ORDER BY u.status_updated_at ASC)
			UNION ALL
				(SELECT ...
						JOIN ...
						WHERE u.status = ?
							AND u.status_updated_at < ?
							AND afe.event = ?
						ORDER BY u.status_updated_at ASC)
			) AS combined_results LIMIT ?
		*/
		query = ``
		for i, eventType := range params.EventTypes {
			queryParams = append(queryParams, console.PendingDeletion, eventType.OlderThan, eventType.EventType)
			if i == 0 {
				query = fmt.Sprintf(`SELECT event, user_id FROM ((%s)`, baseQuery)
				continue
			}
			query += fmt.Sprintf("\n UNION ALL (%s)", baseQuery)

			if i == len(params.EventTypes)-1 {
				query += "\n) AS combined_results LIMIT ?"
			}
		}
		queryParams = append(queryParams, params.Limit)
	} else {
		queryParams = append(queryParams, console.PendingDeletion, params.EventTypes[0].OlderThan, params.EventTypes[0].EventType, params.Limit)
	}

	rows, err := events.db.QueryContext(ctx, events.db.Rebind(query), queryParams...)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	evs := make([]console.EventWithUser, 0, params.Limit)
	for rows.Next() {
		var eventType int
		var userIDBytes []byte

		err = rows.Scan(&eventType, &userIDBytes)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		userID, err := uuid.FromBytes(userIDBytes)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		evs = append(evs, console.EventWithUser{
			UserID: userID,
			Type:   console.AccountFreezeEventType(eventType),
		})
	}

	return evs, rows.Err()
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

// IncrementNotificationsCount is a method for incrementing the notification count for a user's account freeze event.
func (events *accountFreezeEvents) IncrementNotificationsCount(ctx context.Context, userID uuid.UUID, eventType console.AccountFreezeEventType) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = events.db.ExecContext(ctx, events.db.Rebind(`
		UPDATE account_freeze_events
		SET notifications_count = notifications_count + 1
		WHERE user_id = ?
		AND event = ?
	`), userID.Bytes(), int(eventType))

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
