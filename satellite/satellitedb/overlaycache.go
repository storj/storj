// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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

func (cache *overlaycache) SelectStorageNodes(ctx context.Context, count int, criteria *overlay.NodeCriteria) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeType := int(pb.NodeType_STORAGE)

	safeQuery := `
		WHERE NOT disqualified
		AND type = ?
		AND free_bandwidth >= ?
		AND free_disk >= ?
		AND total_audit_count >= ?
		AND total_uptime_count >= ?
		AND last_contact_success > ?
		AND last_contact_success > last_contact_failure`
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
		WHERE NOT disqualified
		AND type = ?
		AND free_bandwidth >= ?
		AND free_disk >= ?
		AND total_audit_count < ?
		AND last_contact_success > ?
		AND last_contact_success > last_contact_failure`
	args := append(make([]interface{}, 0, 10),
		nodeType, criteria.FreeBandwidth, criteria.FreeDisk, criteria.AuditCount, time.Now().Add(-criteria.OnlineWindow))

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
	rows, err = cache.db.Query(cache.db.Rebind(`SELECT id,
	type, address, last_ip, free_bandwidth, free_disk, audit_success_ratio,
	uptime_ratio, total_audit_count, audit_success_count, total_uptime_count,
	uptime_success_count, disqualified, audit_reputation_alpha,
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
			&dbNode.Address, &dbNode.LastIp, &dbNode.FreeBandwidth, &dbNode.FreeDisk,
			&dbNode.AuditSuccessRatio, &dbNode.UptimeRatio,
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

	switch t := cache.db.DB.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		return cache.sqliteQueryNodesDistinct(ctx, excludedNodes, excludedIPs, count, safeQuery, distinctIP, args...)
	case *pq.Driver:
		return cache.postgresQueryNodesDistinct(ctx, excludedNodes, excludedIPs, count, safeQuery, distinctIP, args...)
	default:
		return []*pb.Node{}, Error.New("Unsupported database %t", t)
	}
}

func (cache *overlaycache) sqliteQueryNodesDistinct(ctx context.Context, excludedNodes []storj.NodeID, excludedIPs []string, count int, safeQuery string, distinctIP bool, args ...interface{}) (_ []*pb.Node, err error) {
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
		safeExcludeIPs = ` AND last_ip NOT IN (?` + strings.Repeat(", ?", len(excludedIPs)-1) + `)`
		for _, ip := range excludedIPs {
			args = append(args, ip)
		}
	}

	args = append(args, count)

	rows, err := cache.db.Query(cache.db.Rebind(`SELECT id,
	type, address, last_ip, free_bandwidth, free_disk, audit_success_ratio,
	uptime_ratio, total_audit_count, audit_success_count, total_uptime_count,
	uptime_success_count, disqualified, audit_reputation_alpha,
	audit_reputation_beta, uptime_reputation_alpha, uptime_reputation_beta
	FROM (SELECT id, type, address, last_ip, free_bandwidth, free_disk, audit_success_ratio,
		uptime_ratio, total_audit_count, audit_success_count, total_uptime_count, uptime_success_count, disqualified,
		audit_reputation_alpha, audit_reputation_beta, uptime_reputation_alpha, uptime_reputation_beta,
		Row_number() OVER(PARTITION BY last_ip ORDER BY RANDOM()) rn
		FROM nodes
		`+safeQuery+safeExcludeNodes+safeExcludeIPs+`) n
	WHERE rn = 1
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
			&dbNode.Address, &dbNode.LastIp, &dbNode.FreeBandwidth, &dbNode.FreeDisk,
			&dbNode.AuditSuccessRatio, &dbNode.UptimeRatio,
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

func (cache *overlaycache) postgresQueryNodesDistinct(ctx context.Context, excludedNodes []storj.NodeID, excludedIPs []string, count int, safeQuery string, distinctIP bool, args ...interface{}) (_ []*pb.Node, err error) {
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
		safeExcludeIPs = ` AND last_ip NOT IN (?` + strings.Repeat(", ?", len(excludedIPs)-1) + `)`
		for _, ip := range excludedIPs {
			args = append(args, ip)
		}
	}
	args = append(args, count)

	rows, err := cache.db.Query(cache.db.Rebind(`SELECT DISTINCT ON (last_ip) id,
	type, address, last_ip, free_bandwidth, free_disk, audit_success_ratio,
	uptime_ratio, total_audit_count, audit_success_count, total_uptime_count,
	uptime_success_count, audit_reputation_alpha, audit_reputation_beta, 
	uptime_reputation_alpha, uptime_reputation_beta
	FROM (SELECT id,
		type, address, last_ip, free_bandwidth, free_disk, audit_success_ratio,
		uptime_ratio, total_audit_count, audit_success_count, total_uptime_count,
		uptime_success_count, audit_reputation_alpha, audit_reputation_beta, 
		uptime_reputation_alpha, uptime_reputation_beta
		FROM nodes
		`+safeQuery+safeExcludeNodes+safeExcludeIPs+`
		ORDER BY RANDOM()
		LIMIT ?) n`), args...)

	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()
	var nodes []*pb.Node
	for rows.Next() {
		dbNode := &dbx.Node{}
		err = rows.Scan(&dbNode.Id, &dbNode.Type,
			&dbNode.Address, &dbNode.LastIp, &dbNode.FreeBandwidth, &dbNode.FreeDisk,
			&dbNode.AuditSuccessRatio, &dbNode.UptimeRatio,
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
		return nil, overlay.ErrNodeNotFound.New(id.String())
	}
	if err != nil {
		return nil, err
	}

	return convertDBNode(ctx, node)
}

// IsVetted returns whether or not the node reaches reputable thresholds
func (cache *overlaycache) IsVetted(ctx context.Context, id storj.NodeID, criteria *overlay.NodeCriteria) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	row := cache.db.QueryRow(cache.db.Rebind(`SELECT id
	FROM nodes
	WHERE id = ?
		AND NOT disqualified
		AND type = ?
		AND total_audit_count >= ?
		AND total_uptime_count >= ?
		`), id, pb.NodeType_STORAGE, criteria.AuditCount, criteria.UptimeCount)
	var bytes *[]byte
	err = row.Scan(&bytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// KnownUnreliableOrOffline filters a set of nodes to unreliable or offlines node, independent of new
func (cache *overlaycache) KnownUnreliableOrOffline(ctx context.Context, criteria *overlay.NodeCriteria, nodeIds storj.NodeIDList) (badNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIds) == 0 {
		return nil, Error.New("no ids provided")
	}

	// get reliable and online nodes
	var rows *sql.Rows
	switch t := cache.db.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		args := make([]interface{}, 0, len(nodeIds)+3)
		for i := range nodeIds {
			args = append(args, nodeIds[i].Bytes())
		}
		args = append(args, time.Now().Add(-criteria.OnlineWindow))

		rows, err = cache.db.Query(cache.db.Rebind(`
			SELECT id FROM nodes
			WHERE id IN (?`+strings.Repeat(", ?", len(nodeIds)-1)+`)
			AND NOT disqualified
			AND last_contact_success > ? AND last_contact_success > last_contact_failure
		`), args...)

	case *pq.Driver:
		rows, err = cache.db.Query(`
			SELECT id FROM nodes
				WHERE id = any($1::bytea[])
				AND NOT disqualified
				AND last_contact_success > $2 AND last_contact_success > last_contact_failure
			`, postgresNodeIDList(nodeIds), time.Now().Add(-criteria.OnlineWindow),
		)
	default:
		return nil, Error.New("Unsupported database %t", t)
	}

	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

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

// Update updates node address
func (cache *overlaycache) UpdateAddress(ctx context.Context, info *pb.Node) (err error) {
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
		_, err = tx.Create_Node(
			ctx,
			dbx.Node_Id(info.Id.Bytes()),
			dbx.Node_Address(address.Address),
			dbx.Node_LastIp(info.LastIp),
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
			dbx.Node_AuditSuccessRatio(1),
			dbx.Node_UptimeSuccessCount(0),
			dbx.Node_TotalUptimeCount(0),
			dbx.Node_UptimeRatio(1),
			dbx.Node_LastContactSuccess(time.Now()),
			dbx.Node_LastContactFailure(time.Time{}),
			dbx.Node_Contained(false),
			dbx.Node_Disqualified(false),
			dbx.Node_AuditReputationAlpha(1),
			dbx.Node_AuditReputationBeta(0),
			dbx.Node_UptimeReputationAlpha(1),
			dbx.Node_UptimeReputationBeta(0),
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		update := dbx.Node_Update_Fields{
			Address:  dbx.Node_Address(address.Address),
			LastIp:   dbx.Node_LastIp(info.LastIp),
			Protocol: dbx.Node_Protocol(int(address.Transport)),
		}

		_, err := tx.Update_Node_By_Id(ctx, dbx.Node_Id(info.Id.Bytes()), update)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	return Error.Wrap(tx.Commit())
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
		return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	if startingStats == nil {
		// TODO: add possible sanity checking for alpha and beta values

		updateFields := dbx.Node_Update_Fields{
			TotalAuditCount:       dbx.Node_TotalAuditCount(startingStats.AuditCount),
			TotalUptimeCount:      dbx.Node_TotalUptimeCount(startingStats.UptimeCount),
			AuditReputationAlpha:  dbx.Node_AuditReputationAlpha(startingStats.AuditReputationAlpha),
			AuditReputationBeta:   dbx.Node_AuditReputationBeta(startingStats.AuditReputationBeta),
			UptimeReputationAlpha: dbx.Node_UptimeReputationAlpha(startingStats.UptimeReputationAlpha),
			UptimeReputationBeta:  dbx.Node_UptimeReputationBeta(startingStats.UptimeReputationBeta),
		}

		dbNode, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
		if err != nil {
			return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	// TODO: Allegedly tx.Get_Node_By_Id and tx.Update_Node_By_Id should never return a nil value for dbNode,
	// however we've seen from some crashes that it does. We need to track down the cause of these crashes
	// but for now we're adding a nil check to prevent a panic.
	if dbNode == nil {
		return nil, Error.Wrap(errs.Combine(errs.New("unable to get node by ID: %v", nodeID), tx.Rollback()))
	}
	return getNodeStats(dbNode), Error.Wrap(tx.Commit())
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

	totalAuditCount, auditReputationAlpha, auditReputationBeta := updateReputation(
		updateReq.AuditSuccess,
		dbNode.AuditReputationAlpha,
		dbNode.AuditReputationBeta,
		updateReq.AuditReputationLambda,
		updateReq.AuditReputationWeight,
		dbNode.TotalAuditCount,
	)

	totalUptimeCount, uptimeReputationAlpha, uptimeReputationBeta := updateReputation(
		updateReq.IsUp,
		dbNode.UptimeReputationAlpha,
		dbNode.UptimeReputationBeta,
		updateReq.UptimeReputationLambda,
		updateReq.UptimeReputationWeight,
		dbNode.TotalUptimeCount,
	)

	updateFields := dbx.Node_Update_Fields{
		TotalAuditCount:       dbx.Node_TotalAuditCount(totalAuditCount),
		TotalUptimeCount:      dbx.Node_TotalUptimeCount(totalUptimeCount),
		AuditReputationAlpha:  dbx.Node_AuditReputationAlpha(auditReputationAlpha),
		AuditReputationBeta:   dbx.Node_AuditReputationBeta(auditReputationBeta),
		UptimeReputationAlpha: dbx.Node_UptimeReputationAlpha(uptimeReputationAlpha),
		UptimeReputationBeta:  dbx.Node_UptimeReputationBeta(uptimeReputationBeta),
	}

	if updateReq.IsUp {
		updateFields.LastContactSuccess = dbx.Node_LastContactSuccess(time.Now())
	} else {
		updateFields.LastContactFailure = dbx.Node_LastContactFailure(time.Now())
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

// UpdateNodeInfo updates the email and wallet for a given node ID for satellite payments.
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
			pbts, err := ptypes.Timestamp(nodeInfo.GetVersion().GetTimestamp())
			if err != nil {
				return nil, errs.New("unable to convert version timestamp")
			}
			updateFields.Major = dbx.Node_Major(semVer.Major)
			updateFields.Minor = dbx.Node_Minor(semVer.Minor)
			updateFields.Patch = dbx.Node_Patch(semVer.Patch)
			updateFields.Hash = dbx.Node_Hash(nodeInfo.GetVersion().GetCommitHash())
			updateFields.Timestamp = dbx.Node_Timestamp(pbts)
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
func (cache *overlaycache) UpdateUptime(ctx context.Context, nodeID storj.NodeID, isUp bool, alpha, beta, lambda, weight float64) (stats *overlay.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dbNode, err := tx.Get_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	updateFields := dbx.Node_Update_Fields{}
	updatedUptimeTotal, updatedUptimeAlpha, updatedUptimeBeta := updateReputation(
		isUp,
		alpha,
		beta,
		lambda,
		weight,
		dbNode.TotalUptimeCount,
	)

	updateFields.UptimeReputationAlpha = dbx.Node_UptimeReputationAlpha(updatedUptimeAlpha)
	updateFields.UptimeReputationBeta = dbx.Node_UptimeReputationBeta(updatedUptimeBeta)
	updateFields.TotalUptimeCount = dbx.Node_TotalUptimeCount(updatedUptimeTotal)

	if isUp {
		updateFields.LastContactSuccess = dbx.Node_LastContactSuccess(time.Now())
	} else {
		updateFields.LastContactFailure = dbx.Node_LastContactFailure(time.Now())
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

func convertDBNode(ctx context.Context, info *dbx.Node) (_ *overlay.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)
	if info == nil {
		return nil, Error.New("missing info")
	}

	id, err := storj.NodeIDFromBytes(info.Id)
	if err != nil {
		return nil, err
	}
	ver := &version.SemVer{
		Major: info.Major,
		Minor: info.Minor,
		Patch: info.Patch,
	}

	pbts, err := ptypes.TimestampProto(info.Timestamp)
	if err != nil {
		return nil, err
	}

	node := &overlay.NodeDossier{
		Node: pb.Node{
			Id:     id,
			LastIp: info.LastIp,
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
			Timestamp:  pbts,
			Release:    info.Release,
		},
		Contained:    info.Contained,
		Disqualified: info.Disqualified,
	}

	return node, nil
}

func getNodeStats(dbNode *dbx.Node) *overlay.NodeStats {
	nodeStats := &overlay.NodeStats{
		Latency90:             dbNode.Latency90,
		AuditCount:            dbNode.TotalAuditCount,
		UptimeCount:           dbNode.TotalUptimeCount,
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
func updateReputation(isSuccess bool, alpha, beta, lambda, w float64, totalCount int64) (updatedTotal int64, newAlpha float64, newBeta float64) {
	totalCount++
	// v is a single feedback value that allows us to update both alpha and beta
	var v float64 = -1
	if isSuccess {
		v = 1
	}
	newAlpha = lambda*alpha + w*(1+v)/2
	newBeta = lambda*beta + w*(1-v)/2
	return totalCount, newAlpha, newBeta
}
