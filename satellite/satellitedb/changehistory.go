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

// GetChangesByUserID retrieves the change history for a specific user.
// If exact is false, changes to the user's projects and buckets are also included.
func (c *ChangeHistories) GetChangesByUserID(ctx context.Context, userID uuid.UUID, exact bool) (_ []changehistory.ChangeLog, err error) {
	defer mon.Task()(&ctx)(&err)

	var dbxCHs []*dbx.ChangeHistory
	if exact {
		dbxCHs, err = c.db.All_ChangeHistory_By_UserId_And_ItemType_OrderBy_Desc_Timestamp(ctx,
			dbx.ChangeHistory_UserId(userID.Bytes()),
			dbx.ChangeHistory_ItemType(string(changehistory.ItemTypeUser)),
		)
		if err != nil {
			return nil, err
		}
	} else {
		dbxCHs, err = c.db.All_ChangeHistory_By_UserId_OrderBy_Desc_Timestamp(ctx, dbx.ChangeHistory_UserId(userID.Bytes()))
		if err != nil {
			return nil, err
		}
	}

	return c.convertChangeHistories(dbxCHs)
}

// GetChangesByProjectID retrieves the change history for a specific project.
// If exact is false, changes to the project's buckets are also included.
func (c *ChangeHistories) GetChangesByProjectID(ctx context.Context, projectId uuid.UUID, exact bool) (_ []changehistory.ChangeLog, err error) {
	defer mon.Task()(&ctx)(&err)

	var dbxCHs []*dbx.ChangeHistory
	if exact {
		dbxCHs, err = c.db.All_ChangeHistory_By_ProjectId_And_ItemType_OrderBy_Desc_Timestamp(ctx,
			dbx.ChangeHistory_ProjectId(projectId.Bytes()),
			dbx.ChangeHistory_ItemType(string(changehistory.ItemTypeProject)),
		)
		if err != nil {
			return nil, err
		}
	} else {
		dbxCHs, err = c.db.All_ChangeHistory_By_ProjectId_And_ItemType_Not_OrderBy_Desc_Timestamp(ctx,
			dbx.ChangeHistory_ProjectId(projectId.Bytes()),
			dbx.ChangeHistory_ItemType(string(changehistory.ItemTypeUser)),
		)
		if err != nil {
			return nil, err
		}
	}

	return c.convertChangeHistories(dbxCHs)
}

// GetChangesByBucketName retrieves the change history for a specific bucket.
func (c *ChangeHistories) GetChangesByBucketName(ctx context.Context, bucketName string) (_ []changehistory.ChangeLog, err error) {
	defer mon.Task()(&ctx)(&err)

	var dbxCHs []*dbx.ChangeHistory
	dbxCHs, err = c.db.All_ChangeHistory_By_BucketName_OrderBy_Desc_Timestamp(ctx,
		dbx.ChangeHistory_BucketName([]byte(bucketName)),
	)
	if err != nil {
		return nil, err
	}

	return c.convertChangeHistories(dbxCHs)
}

// TestListChangesByUserID lists change logs for a given user ID, ordered by timestamp descending.
// This method is intended for testing purposes only.
func (c *ChangeHistories) TestListChangesByUserID(ctx context.Context, userID uuid.UUID) (_ []changehistory.ChangeLog, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxCHs, err := c.db.All_ChangeHistory_By_UserId_OrderBy_Desc_Timestamp(ctx, dbx.ChangeHistory_UserId(userID.Bytes()))
	if err != nil {
		return nil, err
	}

	result := make([]changehistory.ChangeLog, 0, len(dbxCHs))
	for _, dbxCH := range dbxCHs {
		ch, err := fromDBXChangeLog(dbxCH)
		if err != nil {
			return nil, err
		}
		result = append(result, *ch)
	}

	return result, nil
}

func (c *ChangeHistories) convertChangeHistories(dbxCHs []*dbx.ChangeHistory) ([]changehistory.ChangeLog, error) {
	result := make([]changehistory.ChangeLog, 0, len(dbxCHs))
	for _, dbxCH := range dbxCHs {
		ch, err := fromDBXChangeLog(dbxCH)
		if err != nil {
			return nil, err
		}
		result = append(result, *ch)
	}

	return result, nil
}

func fromDBXChangeLog(dbxCH *dbx.ChangeHistory) (*changehistory.ChangeLog, error) {
	id, err := uuid.FromBytes(dbxCH.Id)
	if err != nil {
		return nil, err
	}
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
		ID:         id,
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
