// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/golang/protobuf/ptypes"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/pb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type valueattributionDB struct {
	db *dbx.DB
}

// GetByProjectID reads the partner info
func (keys *valueattributionDB) GetByProjectID(ctx context.Context, projectID uuid.UUID) (_ *pb.ConnectorKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxInfo, err := keys.db.Get_ValueAttribution_By_BucketId(ctx, dbx.ValueAttribution_BucketId(projectID[:]))
	if err != nil {
		return nil, err
	}

	createdAt, err := ptypes.TimestampProto(dbxInfo.LastUpdated)
	if err != nil {
		return &pb.ConnectorKeyInfo{}, Error.Wrap(err)
	}

	return &pb.ConnectorKeyInfo{
		PartnerId: dbxInfo.PartnerId,
		BucketId:  dbxInfo.BucketId,
		CreatedAt: createdAt,
	}, nil
}

// Create implements create partner info
func (keys *valueattributionDB) Create(ctx context.Context, info *pb.ConnectorKeyInfo) (connectorkeyinfo *pb.ConnectorKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	createdAt, err := ptypes.Timestamp(info.CreatedAt)
	if err != nil {
		return &pb.ConnectorKeyInfo{}, Error.Wrap(err)
	}

	dbxInfo, err := keys.db.Create_ValueAttribution(ctx, dbx.ValueAttribution_BucketId(info.BucketId),
		dbx.ValueAttribution_PartnerId(info.PartnerId), dbx.ValueAttribution_LastUpdated(createdAt))
	if err != nil {
		return nil, err
	}

	return &pb.ConnectorKeyInfo{
		PartnerId: dbxInfo.PartnerId,
		BucketId:  dbxInfo.BucketId,
		CreatedAt: info.CreatedAt,
	}, nil
}

// Delete implements partner info
func (keys *valueattributionDB) Delete(ctx context.Context, id uuid.UUID) (err error) {
	return nil
}
