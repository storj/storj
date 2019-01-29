// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bandwidthagreement struct {
	db *dbx.DB
}

func (b *bandwidthagreement) CreateAgreement(ctx context.Context, rba *pb.RenterBandwidthAllocation) error {
	expiration := time.Unix(rba.PayerAllocation.ExpirationUnixSec, 0)
	_, err := b.db.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Serialnum(rba.PayerAllocation.SerialNumber+rba.StorageNodeId.String()),
		dbx.Bwagreement_StorageNodeId(rba.StorageNodeId.Bytes()),
		dbx.Bwagreement_UplinkId(rba.PayerAllocation.UplinkId.Bytes()),
		dbx.Bwagreement_Action(int64(rba.PayerAllocation.Action)),
		dbx.Bwagreement_Total(rba.Total),
		dbx.Bwagreement_ExpiresAt(expiration),
	)
	return err
}

//GetTotals returns stats about an uplink
func (b *bandwidthagreement) GetUplinkStats(ctx context.Context, from, to time.Time) (bwa map[storj.NodeID][4]int64, err error) {
	//note:  filter is currently only supported in sqlite and postgres (https://modern-sql.com/feature/filter)
	sql := `SELECT uplink_node, SUM(total), SUM(total) FILTER(action=0), SUM(total) FILTER(action=1), COUNT(*)
		FROM bwagreement WHERE created_at > ? AND created_at <= ? GROUP BY storage_node ORDER BY storage_node`
	rows, err := b.db.DB.Query(sql, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	totals := make(map[storj.NodeID][4]int64)
	for i := 0; rows.Next(); i++ {
		var storageNodeID [len(storj.NodeID{})]byte
		var data [4]int64
		err := rows.Scan(&storageNodeID, &data[0], &data[1], &data[3], &data[3])
		if err != nil {
			return totals, err
		}
		totals[storj.NodeID(storageNodeID)] = data
	}
	return totals, nil
}

//GetTotals returns the sum of each bandwidth type after (exluding) a given date range
func (b *bandwidthagreement) GetTotals(ctx context.Context, from, to time.Time) (bwa map[storj.NodeID][5]int64, err error) {
	//note:  filter is currently only supported in sqlite and postgres (https://modern-sql.com/feature/filter)
	sql := `SELECT storage_node, SUM(total) FILTER(action=0), SUM(total) FILTER(action=1),
	    SUM(total) FILTER(action=2), SUM(total) FILTER(action=3), SUM(total) FILTER(action=4)
		FROM bwagreement WHERE created_at > ? AND created_at <= ? GROUP BY storage_node ORDER BY storage_node`
	rows, err := b.db.DB.Query(sql, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	totals := make(map[storj.NodeID][5]int64)
	for i := 0; rows.Next(); i++ {
		var storageNodeID [len(storj.NodeID{})]byte
		var data [5]int64
		err := rows.Scan(&storageNodeID, &data[0], &data[1], &data[3], &data[3], &data[4])
		if err != nil {
			return totals, err
		}
		totals[storj.NodeID(storageNodeID)] = data
	}
	return totals, nil
}

func (b *bandwidthagreement) DeletePaidAndExpired(ctx context.Context) error {
	// TODO: implement deletion of paid and expired BWAs
	return Error.New("DeletePaidAndExpired not implemented")
}
