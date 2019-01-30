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

const uplinkSQL = `SELECT uplink_id, SUM(total), 
 SUM(CASE WHEN action = 0 THEN total END), 
 SUM(CASE WHEN action = 1 THEN total END), COUNT(*)
FROM bwagreements WHERE created_at > ? 
AND created_at <= ? GROUP BY uplink_id ORDER BY uplink_id`

//GetTotals returns stats about an uplink
func (b *bandwidthagreement) GetUplinkStats(ctx context.Context, from, to time.Time) (bwa map[storj.NodeID][4]int64, err error) {
	rows, err := b.db.DB.Query(uplinkSQL, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	totals := make(map[storj.NodeID][4]int64)
	for i := 0; rows.Next(); i++ {
		var nodeID []byte
		var data [4]int64
		err := rows.Scan(&nodeID, &data[0], &data[1], &data[3], &data[3])
		if err != nil {
			return totals, err
		}
		id, err := storj.NodeIDFromBytes(nodeID)
		if err != nil {
			return totals, err
		}
		totals[id] = data
	}
	return totals, nil
}

const getTotalsSQL = `SELECT storage_node_id, 
 SUM(CASE WHEN action = 0 THEN total END),
 SUM(CASE WHEN action = 1 THEN total END), 
 SUM(CASE WHEN action = 2 THEN total END),
 SUM(CASE WHEN action = 3 THEN total END), 
 SUM(CASE WHEN action = 4 THEN total END)
FROM bwagreements WHERE created_at > ? AND created_at <= ? 
GROUP BY storage_node_id ORDER BY storage_node_id`

//GetTotals returns the sum of each bandwidth type after (exluding) a given date range
func (b *bandwidthagreement) GetTotals(ctx context.Context, from, to time.Time) (bwa map[storj.NodeID][5]int64, err error) {
	rows, err := b.db.DB.Query(getTotalsSQL, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	totals := make(map[storj.NodeID][5]int64)
	for i := 0; rows.Next(); i++ {
		var nodeID []byte
		var data [5]int64
		err := rows.Scan(&nodeID, &data[0], &data[1], &data[3], &data[3], &data[4])
		if err != nil {
			return totals, err
		}
		id, err := storj.NodeIDFromBytes(nodeID)
		if err != nil {
			return totals, err
		}
		totals[id] = data
	}
	return totals, nil
}

func (b *bandwidthagreement) DeletePaidAndExpired(ctx context.Context) error {
	// TODO: implement deletion of paid and expired BWAs
	return Error.New("DeletePaidAndExpired not implemented")
}
