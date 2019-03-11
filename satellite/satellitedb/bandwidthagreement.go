// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type bandwidthagreement struct {
	db *dbx.DB
}

func (b *bandwidthagreement) CreateAgreement(ctx context.Context, rba *pb.Order) (err error) {
	expiration := time.Unix(rba.PayerAllocation.ExpirationUnixSec, 0)
	_, err = b.db.Create_Bwagreement(
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
func (b *bandwidthagreement) GetUplinkStats(ctx context.Context, from, to time.Time) (stats []bwagreement.UplinkStat, err error) {

	var uplinkSQL = fmt.Sprintf(`SELECT uplink_id, SUM(total), 
		COUNT(CASE WHEN action = %d THEN total ELSE null END), 
		COUNT(CASE WHEN action = %d THEN total ELSE null END), COUNT(*)
		FROM bwagreements WHERE created_at > ? 
		AND created_at <= ? GROUP BY uplink_id ORDER BY uplink_id`,
		pb.BandwidthAction_PUT, pb.BandwidthAction_GET)
	rows, err := b.db.DB.Query(b.db.Rebind(uplinkSQL), from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for rows.Next() {
		var nodeID []byte
		stat := bwagreement.UplinkStat{}
		err := rows.Scan(&nodeID, &stat.TotalBytes, &stat.PutActionCount, &stat.GetActionCount, &stat.TotalTransactions)
		if err != nil {
			return stats, err
		}
		id, err := storj.NodeIDFromBytes(nodeID)
		if err != nil {
			return stats, err
		}
		stat.NodeID = id
		stats = append(stats, stat)
	}
	return stats, nil
}

//GetTotals returns the sum of each bandwidth type after (exluding) a given date range
func (b *bandwidthagreement) GetTotals(ctx context.Context, from, to time.Time) (bwa map[storj.NodeID][]int64, err error) {
	var getTotalsSQL = fmt.Sprintf(`SELECT storage_node_id, 
		SUM(CASE WHEN action = %d THEN total ELSE 0 END),
		SUM(CASE WHEN action = %d THEN total ELSE 0 END), 
		SUM(CASE WHEN action = %d THEN total ELSE 0 END),
		SUM(CASE WHEN action = %d THEN total ELSE 0 END), 
		SUM(CASE WHEN action = %d THEN total ELSE 0 END)
		FROM bwagreements WHERE created_at > ? AND created_at <= ? 
		GROUP BY storage_node_id ORDER BY storage_node_id`, pb.BandwidthAction_PUT,
		pb.BandwidthAction_GET, pb.BandwidthAction_GET_AUDIT,
		pb.BandwidthAction_GET_REPAIR, pb.BandwidthAction_PUT_REPAIR)
	rows, err := b.db.DB.Query(b.db.Rebind(getTotalsSQL), from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	totals := make(map[storj.NodeID][]int64)
	for i := 0; rows.Next(); i++ {
		var nodeID []byte
		data := make([]int64, len(pb.BandwidthAction_value))
		err := rows.Scan(&nodeID, &data[pb.BandwidthAction_PUT], &data[pb.BandwidthAction_GET],
			&data[pb.BandwidthAction_GET_AUDIT], &data[pb.BandwidthAction_GET_REPAIR], &data[pb.BandwidthAction_PUT_REPAIR])
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

//GetExpired gets orders that are expired and were created before some time
func (b *bandwidthagreement) GetExpired(before time.Time, expiredAt time.Time) (orders []bwagreement.SavedOrder, err error) {
	var getExpiredSQL = fmt.Sprintf(`SELECT serialnum, storage_node_id, uplink_id, action, total, created_at, expires_at 
		FROM bwagreements WHERE created_at < ? AND expires_at < ?`)
	rows, err := b.db.DB.Query(b.db.Rebind(getExpiredSQL), before, expiredAt)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	for i := 0; rows.Next(); i++ {
		o := bwagreement.SavedOrder{}
		err = rows.Scan(&o.Serialnum, &o.StorageNodeID, &o.UplinkID, &o.Action, &o.Total, &o.CreatedAt, &o.ExpiresAt)
		if err != nil {
			break
		}
		orders = append(orders, o)
	}
	return orders, err
}

//DeleteExpired deletes orders that are expired and were created before some time
func (b *bandwidthagreement) DeleteExpired(before time.Time, expiredAt time.Time) error {
	var deleteExpiredSQL = fmt.Sprintf(`DELETE FROM bwagreements WHERE created_at < ? AND expires_at < ?`)
	_, err := b.db.DB.Exec(b.db.Rebind(deleteExpiredSQL), before, expiredAt)
	return err
}
