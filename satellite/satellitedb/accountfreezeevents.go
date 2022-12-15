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

// Insert is a method for inserting account freeze event into the database.
func (events *accountFreezeEvents) Insert(ctx context.Context, event *console.AccountFreezeEvent) (_ *console.AccountFreezeEvent, err error) {
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

	dbxEvent, err := events.db.Create_AccountFreezeEvent(ctx,
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

// UpdateLimits is a method for updating the limits of an account freeze event by user ID and event type.
func (events *accountFreezeEvents) UpdateLimits(ctx context.Context, userID uuid.UUID, eventType console.AccountFreezeEventType, limits *console.AccountFreezeEventLimits) (err error) {
	defer mon.Task()(&ctx)(&err)

	limitBytes, err := json.Marshal(limits)
	if err != nil {
		return err
	}

	_, err = events.db.Update_AccountFreezeEvent_By_UserId_And_Event(ctx,
		dbx.AccountFreezeEvent_UserId(userID.Bytes()),
		dbx.AccountFreezeEvent_Event(int(eventType)),
		dbx.AccountFreezeEvent_Update_Fields{
			Limits: dbx.AccountFreezeEvent_Limits(limitBytes),
		},
	)

	return err
}

// DeleteAllByUserID is a method for deleting all account freeze events from the database by user ID.
func (events *accountFreezeEvents) DeleteAllByUserID(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = events.db.Delete_AccountFreezeEvent_By_UserId(ctx, dbx.AccountFreezeEvent_UserId(userID.Bytes()))

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
