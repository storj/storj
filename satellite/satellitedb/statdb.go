// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"strings"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var (
	mon             = monkit.Package()
	errAuditSuccess = errs.Class("statdb audit success error")
	errUptime       = errs.Class("statdb uptime error")
)

// StatDB implements the statdb RPC service
type statDB struct {
	db *dbx.DB
}

func getNodeStats(nodeID storj.NodeID, dbNode *dbx.Node) *statdb.NodeStats {
	nodeStats := &statdb.NodeStats{
		NodeID:             nodeID,
		AuditSuccessRatio:  dbNode.AuditSuccessRatio,
		AuditSuccessCount:  dbNode.AuditSuccessCount,
		AuditCount:         dbNode.TotalAuditCount,
		UptimeRatio:        dbNode.UptimeRatio,
		UptimeSuccessCount: dbNode.UptimeSuccessCount,
		UptimeCount:        dbNode.TotalUptimeCount,
	}
	return nodeStats
}

// Create a db entry for the provided storagenode
func (s *statDB) Create(ctx context.Context, nodeID storj.NodeID, startingStats *statdb.NodeStats) (stats *statdb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		totalAuditCount    int64
		auditSuccessCount  int64
		auditSuccessRatio  float64
		totalUptimeCount   int64
		uptimeSuccessCount int64
		uptimeRatio        float64
	)

	if startingStats != nil {
		totalAuditCount = startingStats.AuditCount
		auditSuccessCount = startingStats.AuditSuccessCount
		auditSuccessRatio, err = checkRatioVars(auditSuccessCount, totalAuditCount)
		if err != nil {
			return nil, errAuditSuccess.Wrap(err)
		}

		totalUptimeCount = startingStats.UptimeCount
		uptimeSuccessCount = startingStats.UptimeSuccessCount
		uptimeRatio, err = checkRatioVars(uptimeSuccessCount, totalUptimeCount)
		if err != nil {
			return nil, errUptime.Wrap(err)
		}
	}

	dbNode, err := s.db.Create_Node(
		ctx,
		dbx.Node_Id(nodeID.Bytes()),
		dbx.Node_AuditSuccessCount(auditSuccessCount),
		dbx.Node_TotalAuditCount(totalAuditCount),
		dbx.Node_AuditSuccessRatio(auditSuccessRatio),
		dbx.Node_UptimeSuccessCount(uptimeSuccessCount),
		dbx.Node_TotalUptimeCount(totalUptimeCount),
		dbx.Node_UptimeRatio(uptimeRatio),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, nil
}

// Get a storagenode's stats from the db
func (s *statDB) Get(ctx context.Context, nodeID storj.NodeID) (stats *statdb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	dbNode, err := s.db.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, nil
}

// FindInvalidNodes finds a subset of storagenodes that fail to meet minimum reputation requirements
func (s *statDB) FindInvalidNodes(ctx context.Context, nodeIDs storj.NodeIDList, maxStats *statdb.NodeStats) (invalidIDs storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	var invalidIds storj.NodeIDList

	maxAuditSuccess := maxStats.AuditSuccessRatio
	maxUptime := maxStats.UptimeRatio

	rows, err := s.findInvalidNodesQuery(nodeIDs, maxAuditSuccess, maxUptime)

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

func (s *statDB) findInvalidNodesQuery(nodeIds storj.NodeIDList, auditSuccess, uptime float64) (*sql.Rows, error) {
	args := make([]interface{}, len(nodeIds))
	for i, id := range nodeIds {
		args[i] = id.Bytes()
	}
	args = append(args, auditSuccess, uptime)

	rows, err := s.db.Query(s.db.Rebind(`SELECT nodes.id, nodes.total_audit_count,
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

// Update a single storagenode's stats in the db
func (s *statDB) Update(ctx context.Context, updateReq *statdb.UpdateRequest) (stats *statdb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeID := updateReq.NodeID

	tx, err := s.db.Open(ctx)
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

	updateFields.UptimeSuccessCount = dbx.Node_UptimeSuccessCount(uptimeSuccessCount)
	updateFields.TotalUptimeCount = dbx.Node_TotalUptimeCount(totalUptimeCount)
	updateFields.UptimeRatio = dbx.Node_UptimeRatio(uptimeRatio)

	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, Error.Wrap(tx.Commit())
}

// UpdateUptime updates a single storagenode's uptime stats in the db
func (s *statDB) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (stats *statdb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := s.db.Open(ctx)
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

	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, Error.Wrap(tx.Commit())
}

// UpdateAuditSuccess updates a single storagenode's uptime stats in the db
func (s *statDB) UpdateAuditSuccess(ctx context.Context, nodeID storj.NodeID, auditSuccess bool) (stats *statdb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := s.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	auditSuccessCount := dbNode.AuditSuccessCount
	totalAuditCount := dbNode.TotalAuditCount
	var auditRatio float64

	updateFields := dbx.Node_Update_Fields{}

	auditSuccessCount, totalAuditCount, auditRatio = updateRatioVars(
		auditSuccess,
		auditSuccessCount,
		totalAuditCount,
	)

	updateFields.AuditSuccessCount = dbx.Node_AuditSuccessCount(auditSuccessCount)
	updateFields.TotalAuditCount = dbx.Node_TotalAuditCount(totalAuditCount)
	updateFields.AuditSuccessRatio = dbx.Node_AuditSuccessRatio(auditRatio)

	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	nodeStats := getNodeStats(nodeID, dbNode)
	return nodeStats, Error.Wrap(tx.Commit())
}

// UpdateBatch for updating multiple storage nodes' stats in the db
func (s *statDB) UpdateBatch(ctx context.Context, updateReqList []*statdb.UpdateRequest) (
	statsList []*statdb.NodeStats, failedUpdateReqs []*statdb.UpdateRequest, err error) {
	defer mon.Task()(&ctx)(&err)

	var nodeStatsList []*statdb.NodeStats
	var allErrors []error
	failedUpdateReqs = []*statdb.UpdateRequest{}
	for _, updateReq := range updateReqList {

		nodeStats, err := s.Update(ctx, updateReq)
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

// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
func (s *statDB) CreateEntryIfNotExists(ctx context.Context, nodeID storj.NodeID) (stats *statdb.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	getStats, err := s.Get(ctx, nodeID)
	// TODO: figure out better way to confirm error is type dbx.ErrorCode_NoRows
	if err != nil && strings.Contains(err.Error(), "no rows in result set") {
		createStats, err := s.Create(ctx, nodeID, nil)
		if err != nil {
			return nil, err
		}
		return createStats, nil
	}
	if err != nil {
		return nil, err
	}
	return getStats, nil
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
