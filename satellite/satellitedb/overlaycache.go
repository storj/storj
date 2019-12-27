// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/version"
	"storj.io/storj/satellite/overlay"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()
)

var _ overlay.DB = (*overlaycache)(nil)

type overlaycache struct {
	db *satelliteDB
}

func (cache *overlaycache) SelectStorageNodes(ctx context.Context, count int, criteria *overlay.NodeCriteria) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeType := int(pb.NodeType_STORAGE)

	safeQuery := `
		WHERE disqualified IS NULL
		AND exit_initiated_at IS NULL
		AND type = ?
		AND free_bandwidth >= ?
		AND free_disk >= ?
		AND total_audit_count >= ?
		AND total_uptime_count >= ?
		AND last_contact_success > ?`
	args := append(make([]interface{}, 0, 13),
		nodeType, criteria.FreeBandwidth, criteria.FreeDisk, criteria.AuditCount,
		criteria.UptimeCount, time.Now().Add(-criteria.OnlineWindow))

	if criteria.MinimumVersion != "" {
		v, err := version.NewSemVer(criteria.MinimumVersion)
		if err != nil {
			return nil, Error.New("invalid node selection criteria version: %v", err)
		}
		safeQuery += `
			AND (major > ? OR (major = ? AND (minor > ? OR (minor = ? AND patch >= ?))))
			AND release`
		args = append(args, v.Major, v.Major, v.Minor, v.Minor, v.Patch)
	}

	if !criteria.DistinctIP {
		nodes, err = cache.queryNodes(ctx, criteria.ExcludedNodes, count, safeQuery, args...)
		if err != nil {
			return nil, err
		}
		return nodes, nil
	}

	// query for distinct IPs
	for i := 0; i < 3; i++ {
		moreNodes, err := cache.queryNodesDistinct(ctx, criteria.ExcludedNodes, criteria.ExcludedIPs, count-len(nodes), safeQuery, criteria.DistinctIP, args...)
		if err != nil {
			return nil, err
		}
		for _, n := range moreNodes {
			nodes = append(nodes, n)
			criteria.ExcludedNodes = append(criteria.ExcludedNodes, n.Id)
			criteria.ExcludedIPs = append(criteria.ExcludedIPs, n.LastIp)
		}
		if len(nodes) == count {
			break
		}
	}

	return nodes, nil
}

func (cache *overlaycache) SelectNewStorageNodes(ctx context.Context, count int, criteria *overlay.NodeCriteria) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeType := int(pb.NodeType_STORAGE)

	safeQuery := `
		WHERE disqualified IS NULL
		AND exit_initiated_at IS NULL
		AND type = ?
		AND free_bandwidth >= ?
		AND free_disk >= ?
		AND (total_audit_count < ? OR total_uptime_count < ?)
		AND last_contact_success > ?`
	args := append(make([]interface{}, 0, 10),
		nodeType, criteria.FreeBandwidth, criteria.FreeDisk, criteria.AuditCount, criteria.UptimeCount, time.Now().Add(-criteria.OnlineWindow))

	if criteria.MinimumVersion != "" {
		v, err := version.NewSemVer(criteria.MinimumVersion)
		if err != nil {
			return nil, Error.New("invalid node selection criteria version: %v", err)
		}
		safeQuery += `
			AND (major > ? OR (major = ? AND (minor > ? OR (minor = ? AND patch >= ?))))
			AND release`
		args = append(args, v.Major, v.Major, v.Minor, v.Minor, v.Patch)
	}

	if !criteria.DistinctIP {
		nodes, err = cache.queryNodes(ctx, criteria.ExcludedNodes, count, safeQuery, args...)
		if err != nil {
			return nil, err
		}
		return nodes, nil
	}

	// query for distinct IPs
	for i := 0; i < 3; i++ {
		moreNodes, err := cache.queryNodesDistinct(ctx, criteria.ExcludedNodes, criteria.ExcludedIPs, count-len(nodes), safeQuery, criteria.DistinctIP, args...)
		if err != nil {
			return nil, err
		}
		for _, n := range moreNodes {
			nodes = append(nodes, n)
			criteria.ExcludedNodes = append(criteria.ExcludedNodes, n.Id)
			criteria.ExcludedIPs = append(criteria.ExcludedIPs, n.LastIp)
		}
		if len(nodes) == count {
			break
		}
	}

	return nodes, nil
}

// GetNodeIPs returns a list of node IP addresses. Warning: these node IP addresses might be returned out of order.
func (cache *overlaycache) GetNodeIPs(ctx context.Context, nodeIDs []storj.NodeID) (nodeIPs []string, err error) {
	defer mon.Task()(&ctx)(&err)

	var rows *sql.Rows
	rows, err = cache.db.Query(cache.db.Rebind(`
		SELECT last_net FROM nodes
			WHERE id = any($1::bytea[])
		`), postgresNodeIDList(nodeIDs),
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var ip string
		err = rows.Scan(&ip)
		if err != nil {
			return nil, err
		}
		nodeIPs = append(nodeIPs, ip)
	}
	return nodeIPs, nil
}

func (cache *overlaycache) queryNodes(ctx context.Context, excludedNodes []storj.NodeID, count int, safeQuery string, args ...interface{}) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if count == 0 {
		return nil, nil
	}

	safeExcludeNodes := ""
	if len(excludedNodes) > 0 {
		safeExcludeNodes = ` AND id NOT IN (?` + strings.Repeat(", ?", len(excludedNodes)-1) + `)`
		for _, id := range excludedNodes {
			args = append(args, id.Bytes())
		}
	}

	args = append(args, count)

	var rows *sql.Rows
	rows, err = cache.db.Query(cache.db.Rebind(`SELECT id, type, address, last_net,
	free_bandwidth, free_disk, total_audit_count, audit_success_count,
	total_uptime_count, uptime_success_count, disqualified, audit_reputation_alpha,
	audit_reputation_beta, uptime_reputation_alpha, uptime_reputation_beta
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
			&dbNode.Address, &dbNode.LastNet, &dbNode.FreeBandwidth, &dbNode.FreeDisk,
			&dbNode.TotalAuditCount, &dbNode.AuditSuccessCount,
			&dbNode.TotalUptimeCount, &dbNode.UptimeSuccessCount, &dbNode.Disqualified,
			&dbNode.AuditReputationAlpha, &dbNode.AuditReputationBeta,
			&dbNode.UptimeReputationAlpha, &dbNode.UptimeReputationBeta,
		)
		if err != nil {
			return nil, err
		}

		dossier, err := convertDBNode(ctx, dbNode)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &dossier.Node)
	}

	return nodes, rows.Err()
}

func (cache *overlaycache) queryNodesDistinct(ctx context.Context, excludedNodes []storj.NodeID, excludedIPs []string, count int, safeQuery string, distinctIP bool, args ...interface{}) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if count == 0 {
		return nil, nil
	}

	safeExcludeNodes := ""
	if len(excludedNodes) > 0 {
		safeExcludeNodes = ` AND id NOT IN (?` + strings.Repeat(", ?", len(excludedNodes)-1) + `)`
		for _, id := range excludedNodes {
			args = append(args, id.Bytes())
		}
	}

	safeExcludeIPs := ""
	if len(excludedIPs) > 0 {
		safeExcludeIPs = ` AND last_net NOT IN (?` + strings.Repeat(", ?", len(excludedIPs)-1) + `)`
		for _, ip := range excludedIPs {
			args = append(args, ip)
		}
	}
	args = append(args, count)

	rows, err := cache.db.Query(cache.db.Rebind(`
	SELECT *
	FROM (
		SELECT DISTINCT ON (last_net) last_net,    -- choose at max 1 node from this IP or network
		id, type, address, free_bandwidth, free_disk, total_audit_count,
		audit_success_count, total_uptime_count, uptime_success_count,
		audit_reputation_alpha, audit_reputation_beta, uptime_reputation_alpha,
		uptime_reputation_beta
		FROM nodes
		`+safeQuery+safeExcludeNodes+safeExcludeIPs+`
		AND last_net <> ''                         -- don't try to IP-filter nodes with no known IP yet
		ORDER BY last_net, RANDOM()                -- equal chance of choosing any qualified node at this IP or network
	) filteredcandidates
	ORDER BY RANDOM()                                  -- do the actual node selection from filtered pool
	LIMIT ?`), args...)

	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	var nodes []*pb.Node
	for rows.Next() {
		dbNode := &dbx.Node{}
		err = rows.Scan(&dbNode.LastNet, &dbNode.Id, &dbNode.Type,
			&dbNode.Address, &dbNode.FreeBandwidth, &dbNode.FreeDisk,
			&dbNode.TotalAuditCount, &dbNode.AuditSuccessCount,
			&dbNode.TotalUptimeCount, &dbNode.UptimeSuccessCount,
			&dbNode.AuditReputationAlpha, &dbNode.AuditReputationBeta,
			&dbNode.UptimeReputationAlpha, &dbNode.UptimeReputationBeta,
		)
		if err != nil {
			return nil, err
		}
		dossier, err := convertDBNode(ctx, dbNode)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &dossier.Node)
	}

	return nodes, rows.Err()
}

// Get looks up the node by nodeID
func (cache *overlaycache) Get(ctx context.Context, id storj.NodeID) (_ *overlay.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)

	if id.IsZero() {
		return nil, overlay.ErrEmptyNode
	}

	node, err := cache.db.Get_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	if err == sql.ErrNoRows {
		return nil, overlay.ErrNodeNotFound.New("%v", id)
	}
	if err != nil {
		return nil, err
	}

	return convertDBNode(ctx, node)
}

// KnownOffline filters a set of nodes to offline nodes
func (cache *overlaycache) KnownOffline(ctx context.Context, criteria *overlay.NodeCriteria, nodeIds storj.NodeIDList) (offlineNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIds) == 0 {
		return nil, Error.New("no ids provided")
	}

	// get offline nodes
	var rows *sql.Rows
	rows, err = cache.db.Query(cache.db.Rebind(`
		SELECT id FROM nodes
			WHERE id = any($1::bytea[])
			AND (
				last_contact_success < $2
			)
		`), postgresNodeIDList(nodeIds), time.Now().Add(-criteria.OnlineWindow),
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		offlineNodes = append(offlineNodes, id)
	}
	return offlineNodes, nil
}

// KnownUnreliableOrOffline filters a set of nodes to unreliable or offlines node, independent of new
func (cache *overlaycache) KnownUnreliableOrOffline(ctx context.Context, criteria *overlay.NodeCriteria, nodeIds storj.NodeIDList) (badNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIds) == 0 {
		return nil, Error.New("no ids provided")
	}

	// get reliable and online nodes
	var rows *sql.Rows
	rows, err = cache.db.Query(cache.db.Rebind(`
		SELECT id FROM nodes
			WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND last_contact_success > $2
		`), postgresNodeIDList(nodeIds), time.Now().Add(-criteria.OnlineWindow),
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	goodNodes := make(map[storj.NodeID]struct{}, len(nodeIds))
	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		goodNodes[id] = struct{}{}
	}
	for _, id := range nodeIds {
		if _, ok := goodNodes[id]; !ok {
			badNodes = append(badNodes, id)
		}
	}
	return badNodes, nil
}

// KnownReliable filters a set of nodes to reliable (online and qualified) nodes.
func (cache *overlaycache) KnownReliable(ctx context.Context, onlineWindow time.Duration, nodeIDs storj.NodeIDList) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIDs) == 0 {
		return nil, Error.New("no ids provided")
	}

	// get online nodes
	rows, err := cache.db.Query(cache.db.Rebind(`
		SELECT id, last_net, address, protocol FROM nodes
			WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND last_contact_success > $2
		`), postgresNodeIDList(nodeIDs), time.Now().Add(-onlineWindow),
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		row := &dbx.Id_LastNet_Address_Protocol_Row{}
		err = rows.Scan(&row.Id, &row.LastNet, &row.Address, &row.Protocol)
		if err != nil {
			return nil, err
		}
		node, err := convertDBNodeToPBNode(ctx, row)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// Reliable returns all reliable nodes.
func (cache *overlaycache) Reliable(ctx context.Context, criteria *overlay.NodeCriteria) (nodes storj.NodeIDList, err error) {
	// get reliable and online nodes
	rows, err := cache.db.Query(cache.db.Rebind(`
		SELECT id FROM nodes
		WHERE disqualified IS NULL
		  AND last_contact_success > ?`),
		time.Now().Add(-criteria.OnlineWindow))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, id)
	}
	return nodes, nil
}

// Paginate will run through
func (cache *overlaycache) Paginate(ctx context.Context, offset int64, limit int) (_ []*overlay.NodeDossier, _ bool, err error) {
	defer mon.Task()(&ctx)(&err)

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

	infos := make([]*overlay.NodeDossier, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i], err = convertDBNode(ctx, dbxInfo)
		if err != nil {
			return nil, false, err
		}
	}
	return infos, more, nil
}

// PaginateQualified will retrieve all qualified nodes
func (cache *overlaycache) PaginateQualified(ctx context.Context, offset int64, limit int) (_ []*pb.Node, _ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	cursor := storj.NodeID{}

	// more represents end of table. If there are more rows in the database, more will be true.
	more := true

	if limit <= 0 || limit > storage.LookupLimit {
		limit = storage.LookupLimit
	}

	dbxInfos, err := cache.db.Limited_Node_Id_Node_LastNet_Node_Address_Node_Protocol_By_Id_GreaterOrEqual_And_Disqualified_Is_Null_OrderBy_Asc_Id(ctx, dbx.Node_Id(cursor.Bytes()), limit, offset)
	if err != nil {
		return nil, false, err
	}
	if len(dbxInfos) < limit {
		more = false
	}

	infos := make([]*pb.Node, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i], err = convertDBNodeToPBNode(ctx, dbxInfo)
		if err != nil {
			return nil, false, err
		}
	}
	return infos, more, nil
}

// Update updates node address
func (cache *overlaycache) UpdateAddress(ctx context.Context, info *pb.Node, defaults overlay.NodeSelectionConfig) (err error) {
	defer mon.Task()(&ctx)(&err)

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
		// add the node to DB for first time
		err = tx.CreateNoReturn_Node(
			ctx,
			dbx.Node_Id(info.Id.Bytes()),
			dbx.Node_Address(address.Address),
			dbx.Node_LastNet(info.LastIp),
			dbx.Node_Protocol(int(address.Transport)),
			dbx.Node_Type(int(pb.NodeType_INVALID)),
			dbx.Node_Email(""),
			dbx.Node_Wallet(""),
			dbx.Node_FreeBandwidth(-1),
			dbx.Node_FreeDisk(-1),
			dbx.Node_Major(0),
			dbx.Node_Minor(0),
			dbx.Node_Patch(0),
			dbx.Node_Hash(""),
			dbx.Node_Timestamp(time.Time{}),
			dbx.Node_Release(false),
			dbx.Node_Latency90(0),
			dbx.Node_AuditSuccessCount(0),
			dbx.Node_TotalAuditCount(0),
			dbx.Node_UptimeSuccessCount(0),
			dbx.Node_TotalUptimeCount(0),
			dbx.Node_LastContactSuccess(time.Now()),
			dbx.Node_LastContactFailure(time.Time{}),
			dbx.Node_Contained(false),
			dbx.Node_AuditReputationAlpha(defaults.AuditReputationAlpha0),
			dbx.Node_AuditReputationBeta(defaults.AuditReputationBeta0),
			dbx.Node_UptimeReputationAlpha(defaults.UptimeReputationAlpha0),
			dbx.Node_UptimeReputationBeta(defaults.UptimeReputationBeta0),
			dbx.Node_ExitSuccess(false),
			dbx.Node_Create_Fields{
				Disqualified: dbx.Node_Disqualified_Null(),
			},
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		err = tx.UpdateNoReturn_Node_By_Id(ctx, dbx.Node_Id(info.Id.Bytes()),
			dbx.Node_Update_Fields{
				Address:  dbx.Node_Address(address.Address),
				LastNet:  dbx.Node_LastNet(info.LastIp),
				Protocol: dbx.Node_Protocol(int(address.Transport)),
			})
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	return Error.Wrap(tx.Commit())
}

// BatchUpdateStats updates multiple storagenode's stats in one transaction
func (cache *overlaycache) BatchUpdateStats(ctx context.Context, updateRequests []*overlay.UpdateRequest, batchSize int) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(updateRequests) == 0 {
		return failed, nil
	}

	// ensure updates happen in-order
	sort.Slice(updateRequests, func(i, k int) bool {
		return updateRequests[i].NodeID.Less(updateRequests[k].NodeID)
	})

	doUpdate := func(updateSlice []*overlay.UpdateRequest) (duf storj.NodeIDList, err error) {
		appendAll := func() {
			for _, ur := range updateRequests {
				duf = append(duf, ur.NodeID)
			}
		}

		tx, err := cache.db.Open(ctx)
		if err != nil {
			appendAll()
			return duf, Error.Wrap(err)
		}

		var allSQL string
		for _, updateReq := range updateSlice {
			dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(updateReq.NodeID.Bytes()))
			if err != nil {

				return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
			}

			// do not update reputation if node is disqualified
			if dbNode.Disqualified != nil {
				continue
			}

			updateNodeStats := populateUpdateNodeStats(dbNode, updateReq)
			sql := buildUpdateStatement(updateNodeStats)

			allSQL += sql
		}

		if allSQL != "" {
			results, err := tx.Tx.Exec(allSQL)
			if results == nil || err != nil {
				appendAll()
				return duf, errs.Combine(err, tx.Rollback())
			}

			_, err = results.RowsAffected()
			if err != nil {
				appendAll()
				return duf, errs.Combine(err, tx.Rollback())
			}
		}
		return duf, Error.Wrap(tx.Commit())
	}

	var errlist errs.Group
	length := len(updateRequests)
	for i := 0; i < length; i += batchSize {
		end := i + batchSize
		if end > length {
			end = length
		}

		failedBatch, err := doUpdate(updateRequests[i:end])
		if err != nil && len(failedBatch) > 0 {
			for _, fb := range failedBatch {
				errlist.Add(err)
				failed = append(failed, fb)
			}
		}
	}
	return failed, errlist.Err()
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
		return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}
	// do not update reputation if node is disqualified
	if dbNode.Disqualified != nil {
		return getNodeStats(dbNode), Error.Wrap(tx.Commit())
	}

	updateFields := populateUpdateFields(dbNode, updateReq)

	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	// Cleanup containment table too
	_, err = tx.Delete_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	// TODO: Allegedly tx.Get_Node_By_Id and tx.Update_Node_By_Id should never return a nil value for dbNode,
	// however we've seen from some crashes that it does. We need to track down the cause of these crashes
	// but for now we're adding a nil check to prevent a panic.
	if dbNode == nil {
		return nil, Error.Wrap(errs.Combine(errs.New("unable to get node by ID: %v", nodeID), tx.Rollback()))
	}

	return getNodeStats(dbNode), Error.Wrap(tx.Commit())
}

// UpdateNodeInfo updates the following fields for a given node ID:
// wallet, email for node operator, free disk and bandwidth capacity, and version
func (cache *overlaycache) UpdateNodeInfo(ctx context.Context, nodeID storj.NodeID, nodeInfo *pb.InfoResponse) (stats *overlay.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)

	var updateFields dbx.Node_Update_Fields
	if nodeInfo != nil {
		if nodeInfo.GetType() != pb.NodeType_INVALID {
			updateFields.Type = dbx.Node_Type(int(nodeInfo.GetType()))
		}
		if nodeInfo.GetOperator() != nil {
			updateFields.Wallet = dbx.Node_Wallet(nodeInfo.GetOperator().GetWallet())
			updateFields.Email = dbx.Node_Email(nodeInfo.GetOperator().GetEmail())
		}
		if nodeInfo.GetCapacity() != nil {
			updateFields.FreeDisk = dbx.Node_FreeDisk(nodeInfo.GetCapacity().GetFreeDisk())
			updateFields.FreeBandwidth = dbx.Node_FreeBandwidth(nodeInfo.GetCapacity().GetFreeBandwidth())
		}
		if nodeInfo.GetVersion() != nil {
			semVer, err := version.NewSemVer(nodeInfo.GetVersion().GetVersion())
			if err != nil {
				return nil, errs.New("unable to convert version to semVer")
			}
			updateFields.Major = dbx.Node_Major(int64(semVer.Major))
			updateFields.Minor = dbx.Node_Minor(int64(semVer.Minor))
			updateFields.Patch = dbx.Node_Patch(int64(semVer.Patch))
			updateFields.Hash = dbx.Node_Hash(nodeInfo.GetVersion().GetCommitHash())
			updateFields.Timestamp = dbx.Node_Timestamp(nodeInfo.GetVersion().Timestamp)
			updateFields.Release = dbx.Node_Release(nodeInfo.GetVersion().GetRelease())
		}
	}

	updatedDBNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return convertDBNode(ctx, updatedDBNode)
}

// UpdateUptime updates a single storagenode's uptime stats in the db
func (cache *overlaycache) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool, lambda, weight, uptimeDQ float64) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}
	// do not update reputation if node is disqualified
	if dbNode.Disqualified != nil {
		return getNodeStats(dbNode), Error.Wrap(tx.Commit())
	}

	updateFields := dbx.Node_Update_Fields{}
	uptimeAlpha, uptimeBeta, totalUptimeCount := updateReputation(
		isUp,
		dbNode.UptimeReputationAlpha,
		dbNode.UptimeReputationBeta,
		lambda,
		weight,
		dbNode.TotalUptimeCount,
	)
	mon.FloatVal("uptime_reputation_alpha").Observe(uptimeAlpha)
	mon.FloatVal("uptime_reputation_beta").Observe(uptimeBeta)

	updateFields.UptimeReputationAlpha = dbx.Node_UptimeReputationAlpha(uptimeAlpha)
	updateFields.UptimeReputationBeta = dbx.Node_UptimeReputationBeta(uptimeBeta)
	updateFields.TotalUptimeCount = dbx.Node_TotalUptimeCount(totalUptimeCount)

	uptimeRep := uptimeAlpha / (uptimeAlpha + uptimeBeta)
	if uptimeRep <= uptimeDQ {
		updateFields.Disqualified = dbx.Node_Disqualified(time.Now().UTC())
	}

	lastContactSuccess := dbNode.LastContactSuccess
	lastContactFailure := dbNode.LastContactFailure
	mon.Meter("uptime_updates").Mark(1)
	if isUp {
		updateFields.UptimeSuccessCount = dbx.Node_UptimeSuccessCount(dbNode.UptimeSuccessCount + 1)
		updateFields.LastContactSuccess = dbx.Node_LastContactSuccess(time.Now())

		mon.Meter("uptime_update_successes").Mark(1)
		// we have seen this node in the past 24 hours
		if time.Since(lastContactFailure) > time.Hour*24 {
			mon.Meter("uptime_seen_24h").Mark(1)
		}
		// we have seen this node in the past week
		if time.Since(lastContactFailure) > time.Hour*24*7 {
			mon.Meter("uptime_seen_week").Mark(1)
		}
	} else {
		updateFields.LastContactFailure = dbx.Node_LastContactFailure(time.Now())

		mon.Meter("uptime_update_failures").Mark(1)
		// it's been over 24 hours since we've seen this node
		if time.Since(lastContactSuccess) > time.Hour*24 {
			mon.Meter("uptime_not_seen_24h").Mark(1)
		}
		// it's been over a week since we've seen this node
		if time.Since(lastContactSuccess) > time.Hour*24*7 {
			mon.Meter("uptime_not_seen_week").Mark(1)
		}
	}

	dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}
	// TODO: Allegedly tx.Get_Node_By_Id and tx.Update_Node_By_Id should never return a nil value for dbNode,
	// however we've seen from some crashes that it does. We need to track down the cause of these crashes
	// but for now we're adding a nil check to prevent a panic.
	if dbNode == nil {
		return nil, Error.Wrap(errs.Combine(errs.New("unable to get node by ID: %v", nodeID), tx.Rollback()))
	}

	return getNodeStats(dbNode), Error.Wrap(tx.Commit())
}

// AllPieceCounts returns a map of node IDs to piece counts from the db.
// NB: a valid, partial piece map can be returned even if node ID parsing error(s) are returned.
func (cache *overlaycache) AllPieceCounts(ctx context.Context) (_ map[storj.NodeID]int, err error) {
	defer mon.Task()(&ctx)(&err)

	// NB: `All_Node_Id_Node_PieceCount_By_PieceCount_Not_Number` selects node
	// ID and piece count from the nodes table where piece count is not zero.
	rows, err := cache.db.All_Node_Id_Node_PieceCount_By_PieceCount_Not_Number(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pieceCounts := make(map[storj.NodeID]int)
	nodeIDErrs := errs.Group{}
	for _, row := range rows {
		nodeID, err := storj.NodeIDFromBytes(row.Id)
		if err != nil {
			nodeIDErrs.Add(err)
			continue
		}
		pieceCounts[nodeID] = int(row.PieceCount)
	}

	return pieceCounts, nodeIDErrs.Err()
}

func (cache *overlaycache) UpdatePieceCounts(ctx context.Context, pieceCounts map[storj.NodeID]int) (err error) {
	defer mon.Task()(&ctx)(&err)
	if len(pieceCounts) == 0 {
		return nil
	}

	// TODO: pass in the apprioriate struct to database, rather than constructing it here
	type NodeCount struct {
		ID    storj.NodeID
		Count int64
	}
	var counts []NodeCount

	for nodeid, count := range pieceCounts {
		counts = append(counts, NodeCount{
			ID:    nodeid,
			Count: int64(count),
		})
	}
	sort.Slice(counts, func(i, k int) bool {
		return counts[i].ID.Less(counts[k].ID)
	})

	var nodeIDs []storj.NodeID
	var countNumbers []int64
	for _, count := range counts {
		nodeIDs = append(nodeIDs, count.ID)
		countNumbers = append(countNumbers, count.Count)
	}

	_, err = cache.db.ExecContext(ctx, `
		UPDATE nodes
			SET piece_count = update.count
		FROM (
			SELECT unnest($1::bytea[]) as id, unnest($2::bigint[]) as count
		) as update
		WHERE nodes.id = update.id
	`, postgresNodeIDList(nodeIDs), pq.Array(countNumbers))

	return Error.Wrap(err)
}

// GetExitingNodes returns nodes who have initiated a graceful exit and is not disqualified, but have not completed it.
func (cache *overlaycache) GetExitingNodes(ctx context.Context) (exitingNodes []*overlay.ExitStatus, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(cache.db.Rebind(`
		SELECT id, exit_initiated_at, exit_loop_completed_at, exit_finished_at, exit_success FROM nodes
		WHERE exit_initiated_at IS NOT NULL
		AND exit_finished_at IS NULL
		AND disqualified is NULL
		`),
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var exitingNodeStatus overlay.ExitStatus
		err = rows.Scan(&exitingNodeStatus.NodeID, &exitingNodeStatus.ExitInitiatedAt, &exitingNodeStatus.ExitLoopCompletedAt, &exitingNodeStatus.ExitFinishedAt, &exitingNodeStatus.ExitSuccess)
		if err != nil {
			return nil, err
		}
		exitingNodes = append(exitingNodes, &exitingNodeStatus)
	}
	return exitingNodes, nil
}

// GetExitStatus returns a node's graceful exit status.
func (cache *overlaycache) GetExitStatus(ctx context.Context, nodeID storj.NodeID) (_ *overlay.ExitStatus, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(cache.db.Rebind("select id, exit_initiated_at, exit_loop_completed_at, exit_finished_at, exit_success from nodes where id = ?"), nodeID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()
	exitStatus := &overlay.ExitStatus{}
	if rows.Next() {
		err = rows.Scan(&exitStatus.NodeID, &exitStatus.ExitInitiatedAt, &exitStatus.ExitLoopCompletedAt, &exitStatus.ExitFinishedAt, &exitStatus.ExitSuccess)
	}

	return exitStatus, Error.Wrap(err)
}

// GetGracefulExitCompletedByTimeFrame returns nodes who have completed graceful exit within a time window (time window is around graceful exit completion).
func (cache *overlaycache) GetGracefulExitCompletedByTimeFrame(ctx context.Context, begin, end time.Time) (exitedNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(cache.db.Rebind(`
		SELECT id FROM nodes
		WHERE exit_initiated_at IS NOT NULL
		AND exit_finished_at IS NOT NULL
		AND exit_finished_at >= ?
		AND exit_finished_at < ?
		`), begin, end,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		exitedNodes = append(exitedNodes, id)
	}
	return exitedNodes, rows.Err()
}

// GetGracefulExitIncompleteByTimeFrame returns nodes who have initiated, but not completed graceful exit within a time window (time window is around graceful exit initiation).
func (cache *overlaycache) GetGracefulExitIncompleteByTimeFrame(ctx context.Context, begin, end time.Time) (exitingNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(cache.db.Rebind(`
		SELECT id FROM nodes
		WHERE exit_initiated_at IS NOT NULL
		AND exit_finished_at IS NULL
		AND exit_initiated_at >= ?
		AND exit_initiated_at < ?
		`), begin, end,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	// TODO return more than just ID
	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		exitingNodes = append(exitingNodes, id)
	}
	return exitingNodes, rows.Err()
}

// UpdateExitStatus is used to update a node's graceful exit status.
func (cache *overlaycache) UpdateExitStatus(ctx context.Context, request *overlay.ExitStatusRequest) (_ *overlay.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeID := request.NodeID

	updateFields := populateExitStatusFields(request)

	dbNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if dbNode == nil {
		return nil, Error.Wrap(errs.New("unable to get node by ID: %v", nodeID))
	}

	return convertDBNode(ctx, dbNode)
}

func populateExitStatusFields(req *overlay.ExitStatusRequest) dbx.Node_Update_Fields {
	dbxUpdateFields := dbx.Node_Update_Fields{}

	if !req.ExitInitiatedAt.IsZero() {
		dbxUpdateFields.ExitInitiatedAt = dbx.Node_ExitInitiatedAt(req.ExitInitiatedAt)
	}
	if !req.ExitLoopCompletedAt.IsZero() {
		dbxUpdateFields.ExitLoopCompletedAt = dbx.Node_ExitLoopCompletedAt(req.ExitLoopCompletedAt)
	}
	if !req.ExitFinishedAt.IsZero() {
		dbxUpdateFields.ExitFinishedAt = dbx.Node_ExitFinishedAt(req.ExitFinishedAt)
	}
	dbxUpdateFields.ExitSuccess = dbx.Node_ExitSuccess(req.ExitSuccess)

	return dbxUpdateFields
}

func convertDBNode(ctx context.Context, info *dbx.Node) (_ *overlay.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	if info == nil {
		return nil, Error.New("missing info")
	}

	id, err := storj.NodeIDFromBytes(info.Id)
	if err != nil {
		return nil, err
	}
	ver, err := version.NewSemVer(fmt.Sprintf("%d.%d.%d", info.Major, info.Minor, info.Patch))
	if err != nil {
		return nil, err
	}

	exitStatus := overlay.ExitStatus{NodeID: id}
	exitStatus.ExitInitiatedAt = info.ExitInitiatedAt
	exitStatus.ExitLoopCompletedAt = info.ExitLoopCompletedAt
	exitStatus.ExitFinishedAt = info.ExitFinishedAt

	node := &overlay.NodeDossier{
		Node: pb.Node{
			Id:     id,
			LastIp: info.LastNet,
			Address: &pb.NodeAddress{
				Address:   info.Address,
				Transport: pb.NodeTransport(info.Protocol),
			},
		},
		Type: pb.NodeType(info.Type),
		Operator: pb.NodeOperator{
			Email:  info.Email,
			Wallet: info.Wallet,
		},
		Capacity: pb.NodeCapacity{
			FreeBandwidth: info.FreeBandwidth,
			FreeDisk:      info.FreeDisk,
		},
		Reputation: *getNodeStats(info),
		Version: pb.NodeVersion{
			Version:    ver.String(),
			CommitHash: info.Hash,
			Timestamp:  info.Timestamp,
			Release:    info.Release,
		},
		Contained:    info.Contained,
		Disqualified: info.Disqualified,
		PieceCount:   info.PieceCount,
		ExitStatus:   exitStatus,
		CreatedAt:    info.CreatedAt,
	}

	return node, nil
}

func convertDBNodeToPBNode(ctx context.Context, info *dbx.Id_LastNet_Address_Protocol_Row) (_ *pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	if info == nil {
		return nil, Error.New("missing info")
	}

	id, err := storj.NodeIDFromBytes(info.Id)
	if err != nil {
		return nil, err
	}
	return &pb.Node{
		Id:     id,
		LastIp: info.LastNet,
		Address: &pb.NodeAddress{
			Address:   info.Address,
			Transport: pb.NodeTransport(info.Protocol),
		},
	}, nil
}

func getNodeStats(dbNode *dbx.Node) *overlay.NodeStats {
	nodeStats := &overlay.NodeStats{
		Latency90:             dbNode.Latency90,
		AuditCount:            dbNode.TotalAuditCount,
		AuditSuccessCount:     dbNode.AuditSuccessCount,
		UptimeCount:           dbNode.TotalUptimeCount,
		UptimeSuccessCount:    dbNode.UptimeSuccessCount,
		LastContactSuccess:    dbNode.LastContactSuccess,
		LastContactFailure:    dbNode.LastContactFailure,
		AuditReputationAlpha:  dbNode.AuditReputationAlpha,
		AuditReputationBeta:   dbNode.AuditReputationBeta,
		UptimeReputationAlpha: dbNode.UptimeReputationAlpha,
		UptimeReputationBeta:  dbNode.UptimeReputationBeta,
		Disqualified:          dbNode.Disqualified,
	}
	return nodeStats
}

// updateReputation uses the Beta distribution model to determine a node's reputation.
// lambda is the "forgetting factor" which determines how much past info is kept when determining current reputation score.
// w is the normalization weight that affects how severely new updates affect the current reputation distribution.
func updateReputation(isSuccess bool, alpha, beta, lambda, w float64, totalCount int64) (newAlpha, newBeta float64, updatedCount int64) {
	// v is a single feedback value that allows us to update both alpha and beta
	var v float64 = -1
	if isSuccess {
		v = 1
	}
	newAlpha = lambda*alpha + w*(1+v)/2
	newBeta = lambda*beta + w*(1-v)/2
	return newAlpha, newBeta, totalCount + 1
}

func buildUpdateStatement(update updateNodeStats) string {
	if update.NodeID.IsZero() {
		return ""
	}
	atLeastOne := false
	sql := "UPDATE nodes SET "
	if update.TotalAuditCount.set {
		atLeastOne = true
		sql += fmt.Sprintf("total_audit_count = %v", update.TotalAuditCount.value)
	}
	if update.TotalUptimeCount.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("total_uptime_count = %v", update.TotalUptimeCount.value)
	}
	if update.AuditReputationAlpha.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("audit_reputation_alpha = %v", update.AuditReputationAlpha.value)
	}
	if update.AuditReputationBeta.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("audit_reputation_beta = %v", update.AuditReputationBeta.value)
	}
	if update.UptimeReputationAlpha.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("uptime_reputation_alpha = %v", update.UptimeReputationAlpha.value)
	}
	if update.UptimeReputationBeta.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("uptime_reputation_beta = %v", update.UptimeReputationBeta.value)
	}
	if update.Disqualified.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("disqualified = '%v'", update.Disqualified.value.Format(time.RFC3339Nano))
	}
	if update.UptimeSuccessCount.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("uptime_success_count = %v", update.UptimeSuccessCount.value)
	}
	if update.LastContactSuccess.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("last_contact_success = '%v'", update.LastContactSuccess.value.Format(time.RFC3339Nano))
	}
	if update.LastContactFailure.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("last_contact_failure = '%v'", update.LastContactFailure.value.Format(time.RFC3339Nano))
	}
	if update.AuditSuccessCount.set {
		if atLeastOne {
			sql += ","
		}
		atLeastOne = true
		sql += fmt.Sprintf("audit_success_count = %v", update.AuditSuccessCount.value)
	}
	if update.Contained.set {
		if atLeastOne {
			sql += ","
		}

		atLeastOne = true
		sql += fmt.Sprintf("contained = %v", update.Contained.value)
	}
	if !atLeastOne {
		return ""
	}
	hexNodeID := hex.EncodeToString(update.NodeID.Bytes())

	sql += fmt.Sprintf(" WHERE nodes.id = decode('%v', 'hex');\n", hexNodeID)
	sql += fmt.Sprintf("DELETE FROM pending_audits WHERE pending_audits.node_id = decode('%v', 'hex');\n", hexNodeID)

	return sql
}

type int64Field struct {
	set   bool
	value int64
}

type float64Field struct {
	set   bool
	value float64
}

type boolField struct {
	set   bool
	value bool
}

type timeField struct {
	set   bool
	value time.Time
}

type updateNodeStats struct {
	NodeID                storj.NodeID
	TotalAuditCount       int64Field
	TotalUptimeCount      int64Field
	AuditReputationAlpha  float64Field
	AuditReputationBeta   float64Field
	UptimeReputationAlpha float64Field
	UptimeReputationBeta  float64Field
	Disqualified          timeField
	UptimeSuccessCount    int64Field
	LastContactSuccess    timeField
	LastContactFailure    timeField
	AuditSuccessCount     int64Field
	Contained             boolField
}

func populateUpdateNodeStats(dbNode *dbx.Node, updateReq *overlay.UpdateRequest) updateNodeStats {
	auditAlpha, auditBeta, totalAuditCount := updateReputation(
		updateReq.AuditSuccess,
		dbNode.AuditReputationAlpha,
		dbNode.AuditReputationBeta,
		updateReq.AuditLambda,
		updateReq.AuditWeight,
		dbNode.TotalAuditCount,
	)
	mon.FloatVal("audit_reputation_alpha").Observe(auditAlpha) //locked
	mon.FloatVal("audit_reputation_beta").Observe(auditBeta)   //locked

	uptimeAlpha, uptimeBeta, totalUptimeCount := updateReputation(
		updateReq.IsUp,
		dbNode.UptimeReputationAlpha,
		dbNode.UptimeReputationBeta,
		updateReq.UptimeLambda,
		updateReq.UptimeWeight,
		dbNode.TotalUptimeCount,
	)
	mon.FloatVal("uptime_reputation_alpha").Observe(uptimeAlpha)
	mon.FloatVal("uptime_reputation_beta").Observe(uptimeBeta)

	updateFields := updateNodeStats{
		NodeID:                updateReq.NodeID,
		TotalAuditCount:       int64Field{set: true, value: totalAuditCount},
		TotalUptimeCount:      int64Field{set: true, value: totalUptimeCount},
		AuditReputationAlpha:  float64Field{set: true, value: auditAlpha},
		AuditReputationBeta:   float64Field{set: true, value: auditBeta},
		UptimeReputationAlpha: float64Field{set: true, value: uptimeAlpha},
		UptimeReputationBeta:  float64Field{set: true, value: uptimeBeta},
	}

	auditRep := auditAlpha / (auditAlpha + auditBeta)
	if auditRep <= updateReq.AuditDQ {
		updateFields.Disqualified = timeField{set: true, value: time.Now().UTC()}
	}

	uptimeRep := uptimeAlpha / (uptimeAlpha + uptimeBeta)
	if uptimeRep <= updateReq.UptimeDQ {
		// n.b. that this will overwrite the audit DQ timestamp
		// if it has already been set.
		updateFields.Disqualified = timeField{set: true, value: time.Now().UTC()}
	}

	if updateReq.IsUp {
		updateFields.UptimeSuccessCount = int64Field{set: true, value: dbNode.UptimeSuccessCount + 1}
		updateFields.LastContactSuccess = timeField{set: true, value: time.Now()}
	} else {
		updateFields.LastContactFailure = timeField{set: true, value: time.Now()}
	}

	if updateReq.AuditSuccess {
		updateFields.AuditSuccessCount = int64Field{set: true, value: dbNode.AuditSuccessCount + 1}
	}

	// Updating node stats always exits it from containment mode
	updateFields.Contained = boolField{set: true, value: false}

	return updateFields
}

func populateUpdateFields(dbNode *dbx.Node, updateReq *overlay.UpdateRequest) dbx.Node_Update_Fields {

	update := populateUpdateNodeStats(dbNode, updateReq)
	updateFields := dbx.Node_Update_Fields{}
	if update.TotalAuditCount.set {
		updateFields.TotalAuditCount = dbx.Node_TotalAuditCount(update.TotalAuditCount.value)
	}
	if update.TotalUptimeCount.set {
		updateFields.TotalUptimeCount = dbx.Node_TotalUptimeCount(update.TotalUptimeCount.value)
	}
	if update.AuditReputationAlpha.set {
		updateFields.AuditReputationAlpha = dbx.Node_AuditReputationAlpha(update.AuditReputationAlpha.value)
	}
	if update.AuditReputationBeta.set {
		updateFields.AuditReputationBeta = dbx.Node_AuditReputationBeta(update.AuditReputationBeta.value)
	}
	if update.UptimeReputationAlpha.set {
		updateFields.UptimeReputationAlpha = dbx.Node_UptimeReputationAlpha(update.UptimeReputationAlpha.value)
	}
	if update.UptimeReputationBeta.set {
		updateFields.UptimeReputationBeta = dbx.Node_UptimeReputationBeta(update.UptimeReputationBeta.value)
	}
	if update.Disqualified.set {
		updateFields.Disqualified = dbx.Node_Disqualified(update.Disqualified.value)
	}
	if update.UptimeSuccessCount.set {
		updateFields.UptimeSuccessCount = dbx.Node_UptimeSuccessCount(update.UptimeSuccessCount.value)
	}
	if update.LastContactSuccess.set {
		updateFields.LastContactSuccess = dbx.Node_LastContactSuccess(update.LastContactSuccess.value)
	}
	if update.LastContactFailure.set {
		updateFields.LastContactFailure = dbx.Node_LastContactFailure(update.LastContactFailure.value)
	}
	if update.AuditSuccessCount.set {
		updateFields.AuditSuccessCount = dbx.Node_AuditSuccessCount(update.AuditSuccessCount.value)
	}
	if update.Contained.set {
		updateFields.Contained = dbx.Node_Contained(update.Contained.value)
	}
	if updateReq.AuditSuccess {
		updateFields.AuditSuccessCount = dbx.Node_AuditSuccessCount(dbNode.AuditSuccessCount + 1)
	}

	return updateFields
}

// UpdateCheckIn updates a single storagenode with info from when the the node last checked in.
func (cache *overlaycache) UpdateCheckIn(ctx context.Context, node overlay.NodeCheckInInfo, timestamp time.Time, config overlay.NodeSelectionConfig) (err error) {
	defer mon.Task()(&ctx)(&err)

	if node.Address.GetAddress() == "" {
		return Error.New("error UpdateCheckIn: missing the storage node address")
	}

	// v is a single feedback value that allows us to update both alpha and beta
	var v float64 = -1
	if node.IsUp {
		v = 1
	}

	uptimeReputationAlpha := config.UptimeReputationLambda*config.UptimeReputationAlpha0 + config.UptimeReputationWeight*(1+v)/2
	uptimeReputationBeta := config.UptimeReputationLambda*config.UptimeReputationBeta0 + config.UptimeReputationWeight*(1-v)/2
	semVer, err := version.NewSemVer(node.Version.GetVersion())
	if err != nil {
		return Error.New("unable to convert version to semVer")
	}

	query := `
			INSERT INTO nodes
			(
				id, address, last_net, protocol, type,
				email, wallet, free_bandwidth, free_disk,
				uptime_success_count, total_uptime_count, 
				last_contact_success,
				last_contact_failure,
				audit_reputation_alpha, audit_reputation_beta, uptime_reputation_alpha, uptime_reputation_beta,
				major, minor, patch, hash, timestamp, release
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9,
				$10::bool::int, 1,
				CASE WHEN $10::bool IS TRUE THEN $24::timestamptz
					ELSE '0001-01-01 00:00:00+00'::timestamptz
				END,
				CASE WHEN $10::bool IS FALSE THEN $24::timestamptz
					ELSE '0001-01-01 00:00:00+00'::timestamptz
				END,
				$11, $12, $13, $14,
				$18, $19, $20, $21, $22, $23
			)
			ON CONFLICT (id)
			DO UPDATE
			SET
				address=$2,
				last_net=$3,
				protocol=$4,
				email=$6,
				wallet=$7,
				free_bandwidth=$8,
				free_disk=$9,
				major=$18, minor=$19, patch=$20, hash=$21, timestamp=$22, release=$23,
				total_uptime_count=nodes.total_uptime_count+1,
				uptime_reputation_alpha=$16::float*nodes.uptime_reputation_alpha + $17::float*$10::bool::int::float,
				uptime_reputation_beta=$16::float*nodes.uptime_reputation_beta + $17::float*(NOT $10)::bool::int::float,
				uptime_success_count = nodes.uptime_success_count + $10::bool::int,
				last_contact_success = CASE WHEN $10::bool IS TRUE
					THEN $24::timestamptz
					ELSE nodes.last_contact_success
				END,
				last_contact_failure = CASE WHEN $10::bool IS FALSE
					THEN $24::timestamptz
					ELSE nodes.last_contact_failure
				END,
				-- this disqualified case statement resolves to: 
				-- when (new.uptime_reputation_alpha /(new.uptime_reputation_alpha + new.uptime_reputation_beta)) <= config.UptimeReputationDQ
				disqualified = CASE WHEN (($16::float*nodes.uptime_reputation_alpha + $17::float*$10::bool::int::float) / (($16::float*nodes.uptime_reputation_alpha + $17::float*$10::bool::int::float) + ($16::float*nodes.uptime_reputation_beta + $17::float*(NOT $10)::bool::int::float))) <= $15 AND nodes.disqualified IS NULL
					THEN $24::timestamptz
					ELSE nodes.disqualified
				END;
			`
	_, err = cache.db.ExecContext(ctx, query,
		// args $1 - $5
		node.NodeID.Bytes(), node.Address.GetAddress(), node.LastIP, node.Address.GetTransport(), int(pb.NodeType_STORAGE),
		// args $6 - $9
		node.Operator.GetEmail(), node.Operator.GetWallet(), node.Capacity.GetFreeBandwidth(), node.Capacity.GetFreeDisk(),
		// args $10
		node.IsUp,
		// args $11 - $14
		config.AuditReputationAlpha0, config.AuditReputationBeta0, uptimeReputationAlpha, uptimeReputationBeta,
		// args $15 - $17
		config.UptimeReputationDQ, config.UptimeReputationLambda, config.UptimeReputationWeight,
		// args $18 - $23
		semVer.Major, semVer.Minor, semVer.Patch, node.Version.GetCommitHash(), node.Version.Timestamp, node.Version.GetRelease(),
		// args $24
		timestamp,
	)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
