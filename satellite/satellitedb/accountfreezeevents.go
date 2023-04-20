// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"encoding/json"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// Ensure that accountFreezeEvents implements console.AccountFreezeEvents.
var _ console.AccountFreezeEvents = (*accountFreezeEvents)(nil)

// accountFreezeEvents is an implementation of console.AccountFreezeEvents.
type accountFreezeEvents struct {
	db dbx.Methods
}

// Upsert is a method for updating an account freeze event if it exists and inserting it otherwise.
func (events *accountFreezeEvents) Upsert(ctx context.Context, event *console.AccountFreezeEvent) (_ *console.AccountFreezeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	if event == nil {
		return nil, Error.New("event is nil")
	}

	createFields := dbx.AccountFreezeEvent_Create_Fields{}
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

// GetAll is a method for querying all account freeze events from the database by user ID.
func (events *accountFreezeEvents) GetAll(ctx context.Context, userID uuid.UUID) (freeze *console.AccountFreezeEvent, warning *console.AccountFreezeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	// dbxEvents will have a max length of 2.
	// because there's at most 1 instance each of 2 types of events for a user.
	dbxEvents, err := events.db.All_AccountFreezeEvent_By_UserId(ctx,
		dbx.AccountFreezeEvent_UserId(userID.Bytes()),
	)
	if err != nil {
		return nil, nil, err
	}

	for _, event := range dbxEvents {
		if console.AccountFreezeEventType(event.Event) == console.Freeze {
			freeze, err = fromDBXAccountFreezeEvent(event)
			if err != nil {
				return nil, nil, err
			}
			continue
		}
		warning, err = fromDBXAccountFreezeEvent(event)
		if err != nil {
			return nil, nil, err
		}
	}

	return freeze, warning, nil
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
		UserID:    userID,
		Type:      console.AccountFreezeEventType(dbxEvent.Event),
		CreatedAt: dbxEvent.CreatedAt,
	}
	if dbxEvent.Limits != nil {
		err := json.Unmarshal(dbxEvent.Limits, &event.Limits)
		if err != nil {
			return nil, err
		}
	}
	return event, nil
}
