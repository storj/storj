// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

var (
	mon             = monkit.Package()
	errAuditSuccess = errs.Class("overlay audit success error")
	errUptime       = errs.Class("overlay uptime error")
)

var _ overlay.DB = (*overlaycache)(nil)

type overlaycache struct {
	db *dbx.DB
}

func (cache *overlaycache) SelectStorageNodes(ctx context.Context, count int, criteria *overlay.NodeCriteria) ([]*pb.Node, error) {
	nodeType := int(pb.NodeType_STORAGE)
	return cache.queryFilteredNodes(ctx, criteria.Excluded, count, `
		WHERE type = ? AND free_bandwidth >= ? AND free_disk >= ?
		  AND total_audit_count >= ?
		  AND audit_success_ratio >= ?
		  AND total_uptime_count >= ?
		  AND uptime_ratio >= ?
		  AND last_contact_success > ?
		  AND last_contact_success > last_contact_failure
		`, nodeType, criteria.FreeBandwidth, criteria.FreeDisk,
		criteria.AuditCount, criteria.AuditSuccessRatio, criteria.UptimeCount, criteria.UptimeSuccessRatio,
		time.Now().Add(-1*time.Hour),
	)
}

func (cache *overlaycache) SelectNewStorageNodes(ctx context.Context, count int, criteria *overlay.NewNodeCriteria) ([]*pb.Node, error) {
	nodeType := int(pb.NodeType_STORAGE)
	return cache.queryFilteredNodes(ctx, criteria.Excluded, count, `
		WHERE type = ? AND free_bandwidth >= ? AND free_disk >= ?
		  AND total_audit_count < ?
		  AND last_contact_success > ?
		  AND last_contact_success > last_contact_failure
	`, nodeType, criteria.FreeBandwidth, criteria.FreeDisk,
		criteria.AuditThreshold,
		time.Now().Add(-1*time.Hour),
	)
}

func (cache *overlaycache) queryFilteredNodes(ctx context.Context, excluded []storj.NodeID, count int, safeQuery string, args ...interface{}) (_ []*pb.Node, err error) {
	if count == 0 {
		return nil, nil
	}

	safeExcludeNodes := ""
	if len(excluded) > 0 {
		safeExcludeNodes = ` AND id NOT IN (?` + strings.Repeat(", ?", len(excluded)-1) + `)`
	}
	for _, id := range excluded {
		args = append(args, id.Bytes())
	}
	args = append(args, count)

	rows, err := cache.db.Query(cache.db.Rebind(`SELECT id,
		type, address, free_bandwidth, free_disk, audit_success_ratio,
		uptime_ratio, total_audit_count, audit_success_count, total_uptime_count,
		uptime_success_count
		FROM nodes
		`+safeQuery+safeExcludeNodes+`
		ORDER BY RANDOM()
		LIMIT ?`), args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var nodes []*pb.Node
	for rows.Next() {
		dbNode := &dbx.Node{}
		err = rows.Scan(&dbNode.Id, &dbNode.Type,
			&dbNode.Address, &dbNode.FreeBandwidth, &dbNode.FreeDisk,
			&dbNode.AuditSuccessRatio, &dbNode.UptimeRatio,
			&dbNode.TotalAuditCount, &dbNode.AuditSuccessCount,
			&dbNode.TotalUptimeCount, &dbNode.UptimeSuccessCount)
		if err != nil {
			return nil, err
		}

		node, err := convertDBNode(dbNode)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// Get looks up the node by nodeID
func (cache *overlaycache) Get(ctx context.Context, id storj.NodeID) (*pb.Node, error) {
	if id.IsZero() {
		return nil, overlay.ErrEmptyNode
	}

	node, err := cache.db.Get_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	if err == sql.ErrNoRows {
		return nil, overlay.ErrNodeNotFound.New(id.String())
	}
	if err != nil {
		return nil, err
	}

	return convertDBNode(node)
}

// GetAll looks up nodes based on the ids from the overlay cache
func (cache *overlaycache) GetAll(ctx context.Context, ids storj.NodeIDList) ([]*pb.Node, error) {
	infos := make([]*pb.Node, len(ids))
	for i, id := range ids {
		// TODO: abort on canceled context
		info, err := cache.Get(ctx, id)
		if err != nil {
			continue
		}
		infos[i] = info
	}
	return infos, nil
}

// List lists nodes starting from cursor
func (cache *overlaycache) List(ctx context.Context, cursor storj.NodeID, limit int) ([]*pb.Node, error) {
	// TODO: handle this nicer
	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}

	dbxInfos, err := cache.db.Limited_Node_By_Id_GreaterOrEqual_OrderBy_Asc_Id(ctx, dbx.Node_Id(cursor.Bytes()), limit, 0)
	if err != nil {
		return nil, err
	}

	infos := make([]*pb.Node, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i], err = convertDBNode(dbxInfo)
		if err != nil {
			return nil, err
		}
	}
	return infos, nil
}

// Paginate will run through
func (cache *overlaycache) Paginate(ctx context.Context, offset int64, limit int) ([]*pb.Node, bool, error) {
	cursor := storj.NodeID{}

	// more represents end of table. If there are more rows in the database, more will be true.
	more := true

	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}

	dbxInfos, err := cache.db.Limited_Node_By_Id_GreaterOrEqual_OrderBy_Asc_Id(ctx, dbx.Node_Id(cursor.Bytes()), limit, offset)
	if err != nil {
		return nil, false, err
	}

	if len(dbxInfos) < limit {
		more = false
	}

	infos := make([]*pb.Node, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i], err = convertDBNode(dbxInfo)
		if err != nil {
			return nil, false, err
		}
	}
	return infos, more, nil
}

// Update updates node information
func (cache *overlaycache) Update(ctx context.Context, info *pb.Node) (err error) {
	if info == nil || info.Id.IsZero() {
		return overlay.ErrEmptyNode
	}

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	// TODO: use upsert
	_, err = tx.Get_Node_By_Id(ctx, dbx.Node_Id(info.Id.Bytes()))

	address := info.Address
	if address == nil {
		address = &pb.NodeAddress{}
	}

	if err != nil {
		metadata := info.Metadata
		if metadata == nil {
			metadata = &pb.NodeMetadata{}
		}

		restrictions := info.Restrictions
		if restrictions == nil {
			restrictions = &pb.NodeRestrictions{
				FreeBandwidth: -1,
				FreeDisk:      -1,
			}
		}

		reputation := info.Reputation
		if reputation == nil {
			reputation = &pb.NodeStats{}
		}

		_, err = tx.Create_Node(
			ctx,
			dbx.Node_Id(info.Id.Bytes()),
			dbx.Node_Address(address.Address),
			dbx.Node_Protocol(int(address.Transport)),
			dbx.Node_Type(int(info.Type)),
			dbx.Node_Email(metadata.Email),
			dbx.Node_Wallet(metadata.Wallet),
			dbx.Node_FreeBandwidth(restrictions.FreeBandwidth),
			dbx.Node_FreeDisk(restrictions.FreeDisk),

			dbx.Node_Latency90(reputation.Latency_90),
			dbx.Node_AuditSuccessCount(reputation.AuditSuccessCount),
			dbx.Node_TotalAuditCount(reputation.AuditCount),
			dbx.Node_AuditSuccessRatio(reputation.AuditSuccessRatio),
			dbx.Node_UptimeSuccessCount(reputation.UptimeSuccessCount),
			dbx.Node_TotalUptimeCount(reputation.UptimeCount),
			dbx.Node_UptimeRatio(reputation.UptimeRatio),
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		update := dbx.Node_Update_Fields{
			// TODO: should we be able to update node type?
			Address:  dbx.Node_Address(address.Address),
			Protocol: dbx.Node_Protocol(int(address.Transport)),
		}

		if info.Reputation != nil {
			update.Latency90 = dbx.Node_Latency90(info.Reputation.Latency_90)
			update.AuditSuccessRatio = dbx.Node_AuditSuccessRatio(info.Reputation.AuditSuccessRatio)
			update.UptimeRatio = dbx.Node_UptimeRatio(info.Reputation.UptimeRatio)
			update.TotalAuditCount = dbx.Node_TotalAuditCount(info.Reputation.AuditCount)
			update.AuditSuccessCount = dbx.Node_AuditSuccessCount(info.Reputation.AuditSuccessCount)
			update.TotalUptimeCount = dbx.Node_TotalUptimeCount(info.Reputation.UptimeCount)
			update.UptimeSuccessCount = dbx.Node_UptimeSuccessCount(info.Reputation.UptimeSuccessCount)
		}

		if info.Metadata != nil {
			update.Email = dbx.Node_Email(info.Metadata.Email)
			update.Wallet = dbx.Node_Wallet(info.Metadata.Wallet)
		}

		if info.Restrictions != nil {
			update.FreeBandwidth = dbx.Node_FreeBandwidth(info.Restrictions.FreeBandwidth)
			update.FreeDisk = dbx.Node_FreeDisk(info.Restrictions.FreeDisk)
		}

		_, err := tx.Update_Node_By_Id(ctx, dbx.Node_Id(info.Id.Bytes()), update)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	return Error.Wrap(tx.Commit())
}

// Delete deletes node based on id
func (cache *overlaycache) Delete(ctx context.Context, id storj.NodeID) error {
	_, err := cache.db.Delete_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	return err
}

// CreateStats initializes the stats the provided storagenode
func (cache *overlaycache) CreateStats(ctx context.Context, nodeID storj.NodeID, startingStats *overlay.NodeStats) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	if startingStats != nil {
		auditSuccessRatio, err := checkRatioVars(startingStats.AuditSuccessCount, startingStats.AuditCount)
		if err != nil {
			return nil, errAuditSuccess.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}

		uptimeRatio, err := checkRatioVars(startingStats.UptimeSuccessCount, startingStats.UptimeCount)
		if err != nil {
			return nil, errUptime.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}

		updateFields := dbx.Node_Update_Fields{
			AuditSuccessCount:  dbx.Node_AuditSuccessCount(startingStats.AuditSuccessCount),
			TotalAuditCount:    dbx.Node_TotalAuditCount(startingStats.AuditCount),
			AuditSuccessRatio:  dbx.Node_AuditSuccessRatio(auditSuccessRatio),
			UptimeSuccessCount: dbx.Node_UptimeSuccessCount(startingStats.UptimeSuccessCount),
			TotalUptimeCount:   dbx.Node_TotalUptimeCount(startingStats.UptimeCount),
			UptimeRatio:        dbx.Node_UptimeRatio(uptimeRatio),
		}

		dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
		if err != nil {
			return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	}

	return getNodeStats(nodeID, dbNode), Error.Wrap(tx.Commit())
}

// GetStats a storagenode's stats from the db
func (cache *overlaycache) GetStats(ctx context.Context, nodeID storj.NodeID) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	dbNode, err := cache.db.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err == sql.ErrNoRows {
		return nil, overlay.ErrNodeNotFound.New(nodeID.String())
	}
	if err != nil {
		return nil, Error.Wrap(err)
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, nil
}

// FindInvalidNodes finds a subset of storagenodes that fail to meet minimum reputation requirements
func (cache *overlaycache) FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *overlay.NodeStats) (invalidIDs storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	var invalidIds storj.NodeIDList

	maxAuditSuccess := maxStats.AuditSuccessRatio
	maxUptime := maxStats.UptimeRatio

	rows, err := cache.findInvalidNodesQuery(nodeIDs, maxAuditSuccess, maxUptime)

	if err != nil {
		return nil, err
	}
	defer func() {
		err = utils.CombineErrors(err, rows.Close())
	}()

	for rows.Next() {
		node := &dbx.Node{}
		err = rows.Scan(&node.Id, &node.TotalAuditCount, &node.TotalUptimeCount, &node.AuditSuccessRatio, &node.UptimeRatio)
		if err != nil {
			return nil, err
		}
		id, err := storj.NodeIDFromBytes(node.Id)
		if err != nil {
			return nil, err
		}
		invalidIds = append(invalidIds, id)
	}

	return invalidIds, nil
}

func (cache *overlaycache) findInvalidNodesQuery(nodeIds storj.NodeIDList, auditSuccess, uptime float64) (*sql.Rows, error) {
	args := make([]interface{}, len(nodeIds))
	for i, id := range nodeIds {
		args[i] = id.Bytes()
	}
	args = append(args, auditSuccess, uptime)

	rows, err := cache.db.Query(cache.db.Rebind(`SELECT nodes.id, nodes.total_audit_count,
		nodes.total_uptime_count, nodes.audit_success_ratio,
		nodes.uptime_ratio
		FROM nodes
		WHERE nodes.id IN (?`+strings.Repeat(", ?", len(nodeIds)-1)+`)
		AND nodes.total_audit_count > 0
		AND nodes.total_uptime_count > 0
		AND (
			nodes.audit_success_ratio < ?
			OR nodes.uptime_ratio < ?
		)`), args...)

	return rows, err
}

// UpdateStats a single storagenode's stats in the db
func (cache *overlaycache) UpdateStats(ctx context.Context, updateReq *overlay.UpdateRequest) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeID := updateReq.NodeID

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	auditSuccessCount := dbNode.AuditSuccessCount
	totalAuditCount := dbNode.TotalAuditCount
	var auditSuccessRatio float64
	uptimeSuccessCount := dbNode.UptimeSuccessCount
	totalUptimeCount := dbNode.TotalUptimeCount
	var uptimeRatio float64

	auditSuccessCount, totalAuditCount, auditSuccessRatio = updateRatioVars(
		updateReq.AuditSuccess,
		auditSuccessCount,
		totalAuditCount,
	)

	uptimeSuccessCount, totalUptimeCount, uptimeRatio = updateRatioVars(
		updateReq.IsUp,
		uptimeSuccessCount,
		totalUptimeCount,
	)

	updateFields := dbx.Node_Update_Fields{
		AuditSuccessCount:  dbx.Node_AuditSuccessCount(auditSuccessCount),
		TotalAuditCount:    dbx.Node_TotalAuditCount(totalAuditCount),
		AuditSuccessRatio:  dbx.Node_AuditSuccessRatio(auditSuccessRatio),
		UptimeSuccessCount: dbx.Node_UptimeSuccessCount(uptimeSuccessCount),
		TotalUptimeCount:   dbx.Node_TotalUptimeCount(totalUptimeCount),
		UptimeRatio:        dbx.Node_UptimeRatio(uptimeRatio),
	}

	if updateReq.IsUp {
		updateFields.LastContactSuccess = dbx.Node_LastContactSuccess(time.Now())
	} else {
		updateFields.LastContactFailure = dbx.Node_LastContactFailure(time.Now())
	}

	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, Error.Wrap(tx.Commit())
}

// UpdateOperator updates the email and wallet for a given node ID for satellite payments.
func (cache *overlaycache) UpdateOperator(ctx context.Context, nodeID storj.NodeID, operator pb.NodeOperator) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	updateFields := dbx.Node_Update_Fields{
		Wallet: dbx.Node_Wallet(operator.GetWallet()),
		Email:  dbx.Node_Email(operator.GetEmail()),
	}

	updatedDBNode, err := tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(tx.Rollback())
	}

	updated := getNodeStats(nodeID, updatedDBNode)

	return updated, utils.CombineErrors(err, tx.Commit())
}

// UpdateUptime updates a single storagenode's uptime stats in the db
func (cache *overlaycache) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	uptimeSuccessCount := dbNode.UptimeSuccessCount
	totalUptimeCount := dbNode.TotalUptimeCount
	var uptimeRatio float64

	updateFields := dbx.Node_Update_Fields{}

	uptimeSuccessCount, totalUptimeCount, uptimeRatio = updateRatioVars(
		isUp,
		uptimeSuccessCount,
		totalUptimeCount,
	)

	updateFields.UptimeSuccessCount = dbx.Node_UptimeSuccessCount(uptimeSuccessCount)
	updateFields.TotalUptimeCount = dbx.Node_TotalUptimeCount(totalUptimeCount)
	updateFields.UptimeRatio = dbx.Node_UptimeRatio(uptimeRatio)

	if isUp {
		updateFields.LastContactSuccess = dbx.Node_LastContactSuccess(time.Now())
	} else {
		updateFields.LastContactFailure = dbx.Node_LastContactFailure(time.Now())
	}

	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, Error.Wrap(tx.Commit())
}

// UpdateBatch for updating multiple storage nodes' stats in the db
func (cache *overlaycache) UpdateBatch(ctx context.Context, updateReqList []*overlay.UpdateRequest) (
	statsList []*overlay.NodeStats, failedUpdateReqs []*overlay.UpdateRequest, err error) {
	defer mon.Task()(&ctx)(&err)

	var nodeStatsList []*overlay.NodeStats
	var allErrors []error
	failedUpdateReqs = []*overlay.UpdateRequest{}
	for _, updateReq := range updateReqList {

		nodeStats, err := cache.UpdateStats(ctx, updateReq)
		if err != nil {
			allErrors = append(allErrors, err)
			failedUpdateReqs = append(failedUpdateReqs, updateReq)
		} else {
			nodeStatsList = append(nodeStatsList, nodeStats)
		}
	}

	if len(allErrors) > 0 {
		return nodeStatsList, failedUpdateReqs, Error.Wrap(utils.CombineErrors(allErrors...))
	}
	return nodeStatsList, nil, nil
}

// CreateEntryIfNotExists creates a overlay node entry and saves to overlay if it didn't already exist
func (cache *overlaycache) CreateEntryIfNotExists(ctx context.Context, node *pb.Node) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	getStats, err := cache.GetStats(ctx, node.Id)
	if overlay.ErrNodeNotFound.Has(err) {
		err = cache.Update(ctx, node)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		return cache.GetStats(ctx, node.Id)
	}
	return getStats, err
}

func convertDBNode(info *dbx.Node) (*pb.Node, error) {
	if info == nil {
		return nil, Error.New("missing info")
	}

	id, err := storj.NodeIDFromBytes(info.Id)
	if err != nil {
		return nil, err
	}

	node := &pb.Node{
		Id:   id,
		Type: pb.NodeType(info.Type),
		Address: &pb.NodeAddress{
			Address:   info.Address,
			Transport: pb.NodeTransport(info.Protocol),
		},
		Metadata: &pb.NodeMetadata{
			Email:  info.Email,
			Wallet: info.Wallet,
		},
		Restrictions: &pb.NodeRestrictions{
			FreeBandwidth: info.FreeBandwidth,
			FreeDisk:      info.FreeDisk,
		},
		Reputation: &pb.NodeStats{
			NodeId:             id,
			Latency_90:         info.Latency90,
			AuditSuccessRatio:  info.AuditSuccessRatio,
			UptimeRatio:        info.UptimeRatio,
			AuditCount:         info.TotalAuditCount,
			AuditSuccessCount:  info.AuditSuccessCount,
			UptimeCount:        info.TotalUptimeCount,
			UptimeSuccessCount: info.UptimeSuccessCount,
		},
	}

	if node.Address.Address == "" {
		node.Address = nil
	}
	if node.Metadata.Email == "" && node.Metadata.Wallet == "" {
		node.Metadata = nil
	}
	if node.Restrictions.FreeBandwidth < 0 && node.Restrictions.FreeDisk < 0 {
		node.Restrictions = nil
	}
	if node.Reputation.Latency_90 < 0 {
		node.Reputation = nil
	}

	if time.Now().Sub(info.LastContactSuccess) < 1*time.Hour && info.LastContactSuccess.After(info.LastContactFailure) {
		node.IsUp = true
	}

	return node, nil
}

func getNodeStats(nodeID storj.NodeID, dbNode *dbx.Node) *overlay.NodeStats {
	nodeStats := &overlay.NodeStats{
		NodeID:             nodeID,
		AuditSuccessRatio:  dbNode.AuditSuccessRatio,
		AuditSuccessCount:  dbNode.AuditSuccessCount,
		AuditCount:         dbNode.TotalAuditCount,
		UptimeRatio:        dbNode.UptimeRatio,
		UptimeSuccessCount: dbNode.UptimeSuccessCount,
		UptimeCount:        dbNode.TotalUptimeCount,
		Operator: pb.NodeOperator{
			Email:  dbNode.Email,
			Wallet: dbNode.Wallet,
		},
	}
	return nodeStats
}

func updateRatioVars(newStatus bool, successCount, totalCount int64) (int64, int64, float64) {
	totalCount++
	if newStatus {
		successCount++
	}
	newRatio := float64(successCount) / float64(totalCount)
	return successCount, totalCount, newRatio
}

func checkRatioVars(successCount, totalCount int64) (ratio float64, err error) {
	if successCount < 0 {
		return 0, errs.New("success count less than 0")
	}
	if totalCount < 0 {
		return 0, errs.New("total count less than 0")
	}
	if successCount > totalCount {
		return 0, errs.New("success count greater than total count")
	}
	if totalCount == 0 {
		return 0, nil
	}
	ratio = float64(successCount) / float64(totalCount)
	return ratio, nil
}
