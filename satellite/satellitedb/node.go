// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"strings"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

var (
	mon             = monkit.Package()
	errAuditSuccess = errs.Class("statdb audit success error")
	errUptime       = errs.Class("statdb uptime error")
)

var _ statdb.DB = (*statDB)(nil)

// StatDB implements the statdb RPC service
type statDB struct {
	db *dbx.DB
}

// Create a db entry for the provided storagenode
func (s *statDB) Create(ctx context.Context, nodeID storj.NodeID, startingStats *statdb.NodeStats) (dossier *pb.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		totalAuditCount    int64
		auditSuccessCount  int64
		auditSuccessRatio  float64
		totalUptimeCount   int64
		uptimeSuccessCount int64
		uptimeRatio        float64
		wallet             string
		email              string
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
		wallet = startingStats.Operator.Wallet
		email = startingStats.Operator.Email
	}

	dbNode, err := s.db.Create_Node(
		ctx,
		dbx.Node_Id(nodeID.Bytes()),
		dbx.Node_Address(""), // TODO
		dbx.Node_Protocol(int(pb.NodeTransport_TCP_TLS_GRPC)), // TODO
		dbx.Node_Online(true),                   // TODO
		dbx.Node_Type(int(pb.NodeType_STORAGE)), // TODO
		dbx.Node_Email(email),
		dbx.Node_Wallet(wallet),
		dbx.Node_FreeBandwidth(0), // TODO
		dbx.Node_FreeDisk(0),      // TODO
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

	return convertNode(dbNode)
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
// func (s *statDB) Update(ctx context.Context, updateReq *statdb.UpdateRequest) (stats *statdb.NodeStats, err error) {
// 	defer mon.Task()(&ctx)(&err)

// 	nodeID := updateReq.NodeID

// 	tx, err := s.db.Open(ctx)
// 	if err != nil {
// 		return nil, Error.Wrap(err)
// 	}
// 	dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
// 	if err != nil {
// 		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
// 	}

// 	auditSuccessCount := dbNode.AuditSuccessCount
// 	totalAuditCount := dbNode.TotalAuditCount
// 	var auditSuccessRatio float64
// 	uptimeSuccessCount := dbNode.UptimeSuccessCount
// 	totalUptimeCount := dbNode.TotalUptimeCount
// 	var uptimeRatio float64

// 	auditSuccessCount, totalAuditCount, auditSuccessRatio = updateRatioVars(
// 		updateReq.AuditSuccess,
// 		auditSuccessCount,
// 		totalAuditCount,
// 	)

// 	uptimeSuccessCount, totalUptimeCount, uptimeRatio = updateRatioVars(
// 		updateReq.IsUp,
// 		uptimeSuccessCount,
// 		totalUptimeCount,
// 	)

// 	updateFields := dbx.Node_Update_Fields{
// 		AuditSuccessCount:  dbx.Node_AuditSuccessCount(auditSuccessCount),
// 		TotalAuditCount:    dbx.Node_TotalAuditCount(totalAuditCount),
// 		AuditSuccessRatio:  dbx.Node_AuditSuccessRatio(auditSuccessRatio),
// 		UptimeSuccessCount: dbx.Node_UptimeSuccessCount(uptimeSuccessCount),
// 		TotalUptimeCount:   dbx.Node_TotalUptimeCount(totalUptimeCount),
// 		UptimeRatio:        dbx.Node_UptimeRatio(uptimeRatio),
// 	}

// 	updateFields.UptimeSuccessCount = dbx.Node_UptimeSuccessCount(uptimeSuccessCount)
// 	updateFields.TotalUptimeCount = dbx.Node_TotalUptimeCount(totalUptimeCount)
// 	updateFields.UptimeRatio = dbx.Node_UptimeRatio(uptimeRatio)

// 	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
// 	if err != nil {
// 		return nil, Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
// 	}

// 	nodeStats := getNodeStats(nodeID, dbNode)
// 	return nodeStats, Error.Wrap(tx.Commit())
// }

// UpdateStats takes a NodeStats struct and updates the appropriate node with that information
func (s *statDB) UpdateOperator(ctx context.Context, nodeID storj.NodeID, operator pb.NodeOperator) (dossier *pb.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := s.db.Open(ctx)
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

	dossier, err = convertNode(updatedDBNode)

	return dossier, utils.CombineErrors(err, tx.Commit())
}

// UpdateUptime updates a single storagenode's uptime stats in the db
func (s *statDB) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool) (dossier *pb.NodeDossier, err error) {
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

	dossier, err = convertNode(dbNode)

	return dossier, utils.CombineErrors(err, tx.Commit())
}

// UpdateAuditSuccess updates a single storagenode's uptime stats in the db
func (s *statDB) UpdateAuditSuccess(ctx context.Context, nodeID storj.NodeID, auditSuccess bool) (dossier *pb.NodeDossier, err error) {
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

	dossier, err = convertNode(dbNode)

	return dossier, utils.CombineErrors(err, tx.Commit())
}

// UpdateBatch for updating multiple storage nodes' stats in the db
func (s *statDB) UpdateBatch(ctx context.Context, updateReqList []*statdb.UpdateRequest) (
	statsList []*statdb.NodeStats, failedUpdateReqs []*statdb.UpdateRequest, err error) {
	defer mon.Task()(&ctx)(&err)

	var nodeStatsList []*statdb.NodeStats
	var allErrors []error
	failedUpdateReqs = []*statdb.UpdateRequest{}
	for _, updateReq := range updateReqList {

		// nodeStats, err := s.Update(ctx, updateReq)
		// if err != nil {
		// 	allErrors = append(allErrors, err)
		// 	failedUpdateReqs = append(failedUpdateReqs, updateReq)
		// } else {
		// 	nodeStatsList = append(nodeStatsList, nodeStats)
		// }
	}

	if len(allErrors) > 0 {
		return nodeStatsList, failedUpdateReqs, Error.Wrap(utils.CombineErrors(allErrors...))
	}
	return nodeStatsList, nil, nil
}

// CreateEntryIfNotExists creates a statdb node entry and saves to statdb if it didn't already exist
func (s *statDB) CreateEntryIfNotExists(ctx context.Context, nodeID storj.NodeID) (dossier *pb.NodeDossier, err error) {
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

func (s *statDB) SelectStorageNodes(ctx context.Context, count int, criteria *statdb.NodeCriteria) ([]*pb.Node, error) {
	nodeType := int(pb.NodeType_STORAGE)
	return s.queryFilteredNodes(ctx, criteria.Excluded, count, `
		WHERE type = ? AND free_bandwidth >= ? AND free_disk >= ?
		  AND audit_count >= ?
		  AND audit_success_ratio >= ?
		  AND uptime_count >= ?
		  AND audit_uptime_ratio >= ?
		`, nodeType, criteria.FreeBandwidth, criteria.FreeDisk,
		criteria.AuditCount, criteria.AuditSuccessRatio, criteria.UptimeCount, criteria.UptimeSuccessRatio,
	)
}

func (s *statDB) SelectNewStorageNodes(ctx context.Context, count int, criteria *statdb.NewNodeCriteria) ([]*pb.Node, error) {
	nodeType := int(pb.NodeType_STORAGE)
	return s.queryFilteredNodes(ctx, criteria.Excluded, count,
		`WHERE type = ? AND free_bandwidth >= ? AND free_disk >= ? AND audit_count < ?`,
		nodeType, criteria.FreeBandwidth, criteria.FreeDisk, criteria.AuditThreshold,
	)
}

func (s *statDB) queryFilteredNodes(ctx context.Context, excluded []storj.NodeID, count int, safeQuery string, args ...interface{}) (_ []*pb.Node, err error) {
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

	rows, err := s.db.Query(s.db.Rebind(`SELECT id,
		type, address, free_bandwidth, free_disk, audit_success_ratio,
		audit_uptime_ratio, audit_count, audit_success_count, uptime_count,
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
		dbxNode := &dbx.Node{}
		err = rows.Scan(&dbxNode.Id, &dbxNode.Type,
			&dbxNode.Address, &dbxNode.FreeBandwidth, &dbxNode.FreeDisk,
			&dbxNode.AuditSuccessRatio, &dbxNode.UptimeRatio,
			&dbxNode.TotalAuditCount, &dbxNode.AuditSuccessCount,
			&dbxNode.TotalUptimeCount, &dbxNode.UptimeSuccessCount)
		if err != nil {
			return nil, err
		}

		node, err := convertNode(dbxNode)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node.GetNode())
	}

	return nodes, rows.Err()
}

// Get looks up the node by nodeID
func (s *statDB) Get(ctx context.Context, id storj.NodeID) (*pb.NodeDossier, error) {
	if id.IsZero() {
		return nil, statdb.ErrEmptyNode
	}

	node, err := s.db.Get_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	if err == sql.ErrNoRows {
		return nil, statdb.ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	return convertNode(node)
}

// GetAll looks up nodes based on the ids from the overlay cache
func (s *statDB) GetAll(ctx context.Context, ids storj.NodeIDList) ([]*pb.NodeDossier, error) {
	infos := make([]*pb.NodeDossier, len(ids))
	for i, id := range ids {
		// TODO: abort on canceled context
		info, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		infos[i] = info
	}
	return infos, nil
}

// List lists nodes starting from cursor
func (s *statDB) List(ctx context.Context, cursor storj.NodeID, limit int) ([]*pb.NodeDossier, error) {
	// TODO: handle this nicer
	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}

	dbxInfos, err := s.db.Limited_Node_By_Id_GreaterOrEqual(ctx, dbx.Node_Id(cursor.Bytes()),
		limit, 0,
	)
	if err != nil {
		return nil, err
	}

	infos := make([]*pb.NodeDossier, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i], err = convertNode(dbxInfo)
		if err != nil {
			return nil, err
		}
	}
	return infos, nil
}

// Paginate will run through
func (s *statDB) Paginate(ctx context.Context, offset int64, limit int) ([]*pb.NodeDossier, bool, error) {
	cursor := storj.NodeID{}

	// more represents end of table. If there are more rows in the database, more will be true.
	more := true

	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}

	dbxInfos, err := s.db.Limited_Node_By_Id_GreaterOrEqual(ctx, dbx.Node_Id(cursor.Bytes()), limit, offset)
	if err != nil {
		return nil, false, err
	}

	if len(dbxInfos) < limit {
		more = false
	}

	infos := make([]*pb.NodeDossier, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i], err = convertNode(dbxInfo)
		if err != nil {
			return nil, false, err
		}
	}
	return infos, more, nil
}

// Update updates node information
func (s *statDB) Update(ctx context.Context, info *pb.NodeDossier) (err error) {
	if info == nil || info.GetNode().Id.IsZero() {
		return statdb.ErrEmptyNode
	}

	tx, err := s.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	// TODO: use upsert
	_, err = tx.Get_Node_By_Id(ctx, dbx.Node_Id(info.GetNode().Id.Bytes()))

	address := info.GetNode().Address
	if address == nil {
		address = &pb.NodeAddress{}
	}

	if err != nil {
		// _, err = tx.Create_Node(
		// 	ctx,
		// 	dbx.Node_Id(info.Id.Bytes()),
		// 	dbx.Node_Address(address.Address),
		// 	dbx.Node_Protocol(int(address.Transport)),
		// )
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		update := dbx.Node_Update_Fields{
			Address:  dbx.Node_Address(address.Address),
			Protocol: dbx.Node_Protocol(int(address.Transport)),
		}

		_, err := tx.Update_Node_By_Id(ctx, dbx.Node_Id(info.GetNode().Id.Bytes()), update)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	return Error.Wrap(tx.Commit())
}

// Delete deletes node based on id
func (s *statDB) Delete(ctx context.Context, id storj.NodeID) error {
	_, err := s.db.Delete_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	return err
}

func convertNode(info *dbx.Node) (*pb.NodeDossier, error) {
	if info == nil {
		return nil, Error.New("missing info")
	}

	id, err := storj.NodeIDFromBytes(info.Id)
	if err != nil {
		return nil, err
	}

	dossier := &pb.NodeDossier{
		Node: &pb.Node{
			Id: id,
			Address: &pb.NodeAddress{
				Address:   info.Address,
				Transport: pb.NodeTransport(info.Protocol),
			},
		},
		Online: info.Online,
		Info: &pb.NodeInfo{
			Type: pb.NodeType(info.Type),
			Operator: &pb.NodeOperator{
				Email:  info.Email,
				Wallet: info.Wallet,
			},
			Capacity: &pb.NodeCapacity{
				FreeBandwidth: info.FreeBandwidth,
				FreeDisk:      info.FreeDisk,
			},
		},
		Reputation: &pb.NodeStats{
			NodeId: id, // TODO: remove
			// Latency_90:         info.Latency_90, // TODO: add to db model
			AuditSuccessRatio:  info.AuditSuccessRatio,
			UptimeRatio:        info.UptimeRatio,
			AuditCount:         info.TotalAuditCount,
			AuditSuccessCount:  info.AuditSuccessCount,
			UptimeCount:        info.TotalUptimeCount,
			UptimeSuccessCount: info.UptimeSuccessCount,
		},
	}

	if dossier.GetNode().GetAddress().Address == "" {
		dossier.GetNode().Address = nil
	}

	return dossier, nil
}
