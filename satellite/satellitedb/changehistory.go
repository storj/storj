// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"encoding/json"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ChangeHistories implements changehistory.DB.
type ChangeHistories struct {
	db dbx.DriverMethods
}

var _ changehistory.DB = (*ChangeHistories)(nil)

// LogChange logs a change to the change history.
// the created ChangeLog is returned mostly for testing purposes.
func (c *ChangeHistories) LogChange(ctx context.Context, params changehistory.ChangeLog) (_ *changehistory.ChangeLog, err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := uuid.New()
	if err != nil {
		return nil, err
	}
	fields := dbx.ChangeHistory_Create_Fields{}
	if params.ProjectID != nil {
		fields.ProjectId = dbx.ChangeHistory_ProjectId(params.ProjectID.Bytes())
	} else {
		fields.ProjectId = dbx.ChangeHistory_ProjectId_Null()
	}
	if params.BucketName != nil {
		fields.BucketName = dbx.ChangeHistory_BucketName([]byte(*params.BucketName))
	} else {
		fields.BucketName = dbx.ChangeHistory_BucketName_Null()
	}

	fields.Timestamp = dbx.ChangeHistory_Timestamp(params.Timestamp)

	changesJson, err := json.Marshal(params.Changes)
	if err != nil {
		return nil, err
	}

	cH, err := c.db.Create_ChangeHistory(
		ctx,
		dbx.ChangeHistory_Id(id.Bytes()),
		dbx.ChangeHistory_AdminEmail(params.AdminEmail),
		dbx.ChangeHistory_UserId(params.UserID.Bytes()),
		dbx.ChangeHistory_ItemType(string(params.ItemType)),
		dbx.ChangeHistory_Operation(params.Operation),
		dbx.ChangeHistory_Reason(params.Reason),
		dbx.ChangeHistory_Changes(changesJson),
		fields,
	)
	if err != nil {
		return nil, err
	}

	return fromDBXChangeLog(cH)
}

// TestListChangesByUserID lists change logs for a given user ID, ordered by timestamp descending.
// This method is intended for testing purposes only.
func (c *ChangeHistories) TestListChangesByUserID(ctx context.Context, userID uuid.UUID) (_ []*changehistory.ChangeLog, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxCHs, err := c.db.All_ChangeHistory_By_UserId_OrderBy_Desc_Timestamp(ctx, dbx.ChangeHistory_UserId(userID.Bytes()))
	if err != nil {
		return nil, err
	}

	result := make([]*changehistory.ChangeLog, 0, len(dbxCHs))
	for _, dbxCH := range dbxCHs {
		ch, err := fromDBXChangeLog(dbxCH)
		if err != nil {
			return nil, err
		}
		result = append(result, ch)
	}

	return result, nil
}

func fromDBXChangeLog(dbxCH *dbx.ChangeHistory) (*changehistory.ChangeLog, error) {
	userID, err := uuid.FromBytes(dbxCH.UserId)
	if err != nil {
		return nil, err
	}
	var projectId *uuid.UUID
	if dbxCH.ProjectId != nil {
		projectId = new(uuid.UUID)
		*projectId, err = uuid.FromBytes(dbxCH.ProjectId)
		if err != nil {
			return nil, err
		}
	}

	var changes map[string]any
	if len(dbxCH.Changes) > 0 {
		if err := json.Unmarshal(dbxCH.Changes, &changes); err != nil {
			return nil, err
		}
	} else {
		changes = make(map[string]any)
	}

	cl := &changehistory.ChangeLog{
		UserID:     userID,
		ProjectID:  projectId,
		AdminEmail: dbxCH.AdminEmail,
		ItemType:   changehistory.ItemType(dbxCH.ItemType),
		Reason:     dbxCH.Reason,
		Operation:  dbxCH.Operation,
		Changes:    changes,
		Timestamp:  dbxCH.Timestamp,
	}
	if dbxCH.BucketName != nil {
		bucketName := string(dbxCH.BucketName)
		cl.BucketName = &bucketName
	}
	return cl, nil
}
