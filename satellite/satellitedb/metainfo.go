// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type metainfo struct {
	db *dbx.DB
}

func (m *metainfo) CreateBucket(ctx context.Context, info *storj.Bucket) (storj.Bucket, error) {
	dbxInfo, err := m.db.Create_Bucket(
		ctx,
		dbx.Bucket_Name(info.Name),
		dbx.Bucket_CreatedAt(info.Created),
		dbx.Bucket_PathCipher(int(info.PathCipher)),
	)
	return convertInfo(dbxInfo), err
}

func (m *metainfo) DeleteBucket(ctx context.Context, bucket string) error {
	// TODO: check if bucket is empty
	_, err := m.db.Delete_Bucket_By_Name(
		ctx,
		dbx.Bucket_Name(bucket),
	)
	return err
}

func (m *metainfo) GetBucket(ctx context.Context, bucket string) (storj.Bucket, error) {
	dbxInfo, err := m.db.Get_Bucket_By_Name(
		ctx,
		dbx.Bucket_Name(bucket),
	)
	return convertInfo(dbxInfo), err
}

func (m *metainfo) ListBuckets(ctx context.Context, options storj.BucketListOptions) (storj.BucketList, error) {
	var query func(ctx context.Context, bucket_name_greater dbx.Bucket_Name_Field, limit int, offset int64) (rows []*dbx.Bucket, err error)

	switch options.Direction {
	case storj.Before:
		query = m.db.Limited_Bucket_By_Name_Less_OrderBy_Desc_Name
	case storj.Backward:
		query = m.db.Limited_Bucket_By_Name_LessOrEqual_OrderBy_Desc_Name
	case storj.Forward:
		query = m.db.Limited_Bucket_By_Name_GreaterOrEqual_OrderBy_Asc_Name
	case storj.After:
		query = m.db.Limited_Bucket_By_Name_Greater_OrderBy_Asc_Name
	default:
		return storj.BucketList{}, errs.New("unknown direction: %d", options.Direction)
	}

	rows, err := query(
		ctx,
		dbx.Bucket_Name(options.Cursor),
		options.Limit+1,
		0,
	)
	if err != nil {
		return storj.BucketList{}, err
	}

	if options.Direction == storj.Before || options.Direction == storj.Backward {
		reverse(rows)
	}

	list := storj.BucketList{
		More: len(rows) > options.Limit,
	}

	if list.More {
		rows = rows[:options.Limit]
	}

	list.Items = make([]storj.Bucket, len(rows))

	for i := 0; i < len(rows); i++ {
		list.Items[i] = convertInfo(rows[i])
	}

	return list, nil
}

func convertInfo(dbxInfo *dbx.Bucket) storj.Bucket {
	if dbxInfo == nil {
		return storj.Bucket{}
	}
	return storj.Bucket{
		Name:       dbxInfo.Name,
		Created:    dbxInfo.CreatedAt,
		PathCipher: storj.Cipher(dbxInfo.PathCipher),
	}
}

func reverse(rows []*dbx.Bucket) {
	for i := len(rows)/2 - 1; i >= 0; i-- {
		opp := len(rows) - 1 - i
		rows[i], rows[opp] = rows[opp], rows[i]
	}
}
