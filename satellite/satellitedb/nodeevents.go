// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/satellitedb/dbx"
)

var _ nodeevents.DB = (*nodeEvents)(nil)

type nodeEvents struct {
	db *satelliteDB
}

// Insert a node event into the node events table.
func (ne *nodeEvents) Insert(ctx context.Context, email string, nodeID storj.NodeID, eventType nodeevents.Type) (nodeEvent nodeevents.NodeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := uuid.New()
	if err != nil {
		return nodeEvent, err
	}
	entry, err := ne.db.Create_NodeEvent(ctx, dbx.NodeEvent_Id(id.Bytes()), dbx.NodeEvent_Email(email), dbx.NodeEvent_NodeId(nodeID.Bytes()), dbx.NodeEvent_Event(int(eventType)), dbx.NodeEvent_Create_Fields{})
	if err != nil {
		return nodeEvent, err
	}

	ne.db.log.Info("node event inserted", zap.Int("type", int(eventType)), zap.String("email", email), zap.String("node ID", nodeID.String()))

	return fromDBX(entry)
}

// GetLatestByEmailAndEvent gets latest node event by email and event type.
func (ne *nodeEvents) GetLatestByEmailAndEvent(ctx context.Context, email string, event nodeevents.Type) (nodeEvent nodeevents.NodeEvent, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxNE, err := ne.db.Get_NodeEvent_By_Email_And_Event_OrderBy_Desc_CreatedAt(ctx, dbx.NodeEvent_Email(email), dbx.NodeEvent_Event(int(event)))
	if err != nil {
		return nodeEvent, err
	}
	return fromDBX(dbxNE)
}

func fromDBX(dbxNE *dbx.NodeEvent) (event nodeevents.NodeEvent, err error) {
	id, err := uuid.FromBytes(dbxNE.Id)
	if err != nil {
		return event, err
	}
	nodeID, err := storj.NodeIDFromBytes(dbxNE.NodeId)
	if err != nil {
		return event, err
	}
	return nodeevents.NodeEvent{
		ID:        id,
		Email:     dbxNE.Email,
		NodeID:    nodeID,
		Event:     nodeevents.Type(dbxNE.Event),
		CreatedAt: dbxNE.CreatedAt,
		EmailSent: dbxNE.EmailSent,
	}, nil
}
