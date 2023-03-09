// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/private/dbutil/cockroachutil"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/tagsql"
	"storj.io/private/version"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/dbx"
)

var (
	mon = monkit.Package()
)

var _ overlay.DB = (*overlaycache)(nil)

type overlaycache struct {
	db *satelliteDB
}

// SelectAllStorageNodesUpload returns all nodes that qualify to store data, organized as reputable nodes and new nodes.
func (cache *overlaycache) SelectAllStorageNodesUpload(ctx context.Context, selectionCfg overlay.NodeSelectionConfig) (reputable, new []*overlay.SelectedNode, err error) {
	for {
		reputable, new, err = cache.selectAllStorageNodesUpload(ctx, selectionCfg)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return reputable, new, err
		}
		break
	}

	return reputable, new, err
}

func (cache *overlaycache) selectAllStorageNodesUpload(ctx context.Context, selectionCfg overlay.NodeSelectionConfig) (reputable, new []*overlay.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT id, address, last_net, last_ip_port, vetted_at, country_code, noise_proto, noise_public_key
			FROM nodes
			` + cache.db.impl.AsOfSystemInterval(selectionCfg.AsOfSystemTime.Interval()) + `
			WHERE disqualified IS NULL
			AND unknown_audit_suspended IS NULL
			AND offline_suspended IS NULL
			AND exit_initiated_at IS NULL
			AND type = $1
			AND free_disk >= $2
			AND last_contact_success > $3
	`
	args := []interface{}{
		// $1
		int(pb.NodeType_STORAGE),
		// $2
		selectionCfg.MinimumDiskSpace.Int64(),
		// $3
		time.Now().Add(-selectionCfg.OnlineWindow),
	}
	if selectionCfg.MinimumVersion != "" {
		version, err := version.NewSemVer(selectionCfg.MinimumVersion)
		if err != nil {
			return nil, nil, err
		}
		query += `AND (major > $4 OR (major = $5 AND (minor > $6 OR (minor = $7 AND patch >= $8)))) AND release`
		args = append(args,
			// $4 - $8
			version.Major, version.Major, version.Minor, version.Minor, version.Patch,
		)
	}

	rows, err := cache.db.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var reputableNodes []*overlay.SelectedNode
	var newNodes []*overlay.SelectedNode
	for rows.Next() {
		var node overlay.SelectedNode
		node.Address = &pb.NodeAddress{}
		var lastIPPort sql.NullString
		var vettedAt *time.Time
		var noise noiseScanner
		err = rows.Scan(&node.ID, &node.Address.Address, &node.LastNet, &lastIPPort, &vettedAt, &node.CountryCode, &noise.Proto, &noise.PublicKey)
		if err != nil {
			return nil, nil, err
		}
		if lastIPPort.Valid {
			node.LastIPPort = lastIPPort.String
		}
		node.Address.NoiseInfo = noise.Convert()

		if vettedAt == nil {
			newNodes = append(newNodes, &node)
			continue
		}
		reputableNodes = append(reputableNodes, &node)
	}

	return reputableNodes, newNodes, Error.Wrap(rows.Err())
}

// SelectAllStorageNodesDownload returns all nodes that qualify to store data, organized as reputable nodes and new nodes.
func (cache *overlaycache) SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf overlay.AsOfSystemTimeConfig) (nodes []*overlay.SelectedNode, err error) {
	for {
		nodes, err = cache.selectAllStorageNodesDownload(ctx, onlineWindow, asOf)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nodes, err
		}
		break
	}

	return nodes, err
}

func (cache *overlaycache) selectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOfConfig overlay.AsOfSystemTimeConfig) (_ []*overlay.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `
		SELECT id, address, last_net, last_ip_port, noise_proto, noise_public_key
			FROM nodes
			` + cache.db.impl.AsOfSystemInterval(asOfConfig.Interval()) + `
			WHERE disqualified IS NULL
			AND exit_finished_at IS NULL
			AND last_contact_success > $1
	`
	args := []interface{}{
		// $1
		time.Now().Add(-onlineWindow),
	}

	rows, err := cache.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var nodes []*overlay.SelectedNode
	for rows.Next() {
		var node overlay.SelectedNode
		node.Address = &pb.NodeAddress{}
		var lastIPPort sql.NullString
		var noise noiseScanner
		err = rows.Scan(&node.ID, &node.Address.Address, &node.LastNet, &lastIPPort, &noise.Proto, &noise.PublicKey)
		if err != nil {
			return nil, err
		}
		if lastIPPort.Valid {
			node.LastIPPort = lastIPPort.String
		}
		node.Address.NoiseInfo = noise.Convert()
		nodes = append(nodes, &node)
	}
	return nodes, Error.Wrap(rows.Err())
}

// GetNodesNetwork returns the /24 subnet for each storage node, order is not guaranteed.
func (cache *overlaycache) GetNodesNetwork(ctx context.Context, nodeIDs []storj.NodeID) (nodeNets []string, err error) {
	for {
		nodeNets, err = cache.getNodesNetwork(ctx, nodeIDs)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nodeNets, err
		}
		break
	}

	return nodeNets, err
}

func (cache *overlaycache) getNodesNetwork(ctx context.Context, nodeIDs []storj.NodeID) (nodeNets []string, err error) {
	defer mon.Task()(&ctx)(&err)

	var rows tagsql.Rows
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
		SELECT last_net FROM nodes
			WHERE id = any($1::bytea[])
		`), pgutil.NodeIDArray(nodeIDs),
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
		nodeNets = append(nodeNets, ip)
	}
	return nodeNets, Error.Wrap(rows.Err())
}

// Get looks up the node by nodeID.
func (cache *overlaycache) Get(ctx context.Context, id storj.NodeID) (dossier *overlay.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)

	if id.IsZero() {
		return nil, overlay.ErrEmptyNode
	}

	node, err := cache.db.Get_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, overlay.ErrNodeNotFound.New("%v", id)
	}
	if err != nil {
		return nil, err
	}

	return convertDBNode(ctx, node)
}

// GetOnlineNodesForGetDelete returns a map of nodes for the supplied nodeIDs.
func (cache *overlaycache) GetOnlineNodesForGetDelete(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration, asOf overlay.AsOfSystemTimeConfig) (nodes map[storj.NodeID]*overlay.SelectedNode, err error) {
	for {
		nodes, err = cache.getOnlineNodesForGetDelete(ctx, nodeIDs, onlineWindow, asOf)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nodes, err
		}
		break
	}

	return nodes, err
}

func (cache *overlaycache) getOnlineNodesForGetDelete(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration, asOf overlay.AsOfSystemTimeConfig) (_ map[storj.NodeID]*overlay.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	var rows tagsql.Rows
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
		SELECT last_net, id, address, last_ip_port, noise_proto, noise_public_key
		FROM nodes
		`+cache.db.impl.AsOfSystemInterval(asOf.Interval())+`
		WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND exit_finished_at IS NULL
			AND last_contact_success > $2
	`), pgutil.NodeIDArray(nodeIDs), time.Now().Add(-onlineWindow))
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	nodes := make(map[storj.NodeID]*overlay.SelectedNode)
	for rows.Next() {
		var node overlay.SelectedNode
		node.Address = &pb.NodeAddress{}

		var lastIPPort sql.NullString
		var noise noiseScanner
		err = rows.Scan(&node.LastNet, &node.ID, &node.Address.Address, &lastIPPort, &noise.Proto, &noise.PublicKey)
		if err != nil {
			return nil, err
		}
		if lastIPPort.Valid {
			node.LastIPPort = lastIPPort.String
		}
		node.Address.NoiseInfo = noise.Convert()

		nodes[node.ID] = &node
	}

	return nodes, Error.Wrap(rows.Err())
}

// GetOnlineNodesForAuditRepair returns a map of nodes for the supplied nodeIDs.
func (cache *overlaycache) GetOnlineNodesForAuditRepair(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration) (nodes map[storj.NodeID]*overlay.NodeReputation, err error) {
	for {
		nodes, err = cache.getOnlineNodesForAuditRepair(ctx, nodeIDs, onlineWindow)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nodes, err
		}
		break
	}

	return nodes, err
}

func (cache *overlaycache) getOnlineNodesForAuditRepair(ctx context.Context, nodeIDs []storj.NodeID, onlineWindow time.Duration) (_ map[storj.NodeID]*overlay.NodeReputation, err error) {
	defer mon.Task()(&ctx)(&err)

	var rows tagsql.Rows
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
		SELECT last_net, id, address, email, last_ip_port, noise_proto, noise_public_key, vetted_at,
			unknown_audit_suspended, offline_suspended
		FROM nodes
		WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND exit_finished_at IS NULL
			AND last_contact_success > $2
	`), pgutil.NodeIDArray(nodeIDs), time.Now().Add(-onlineWindow))
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	nodes := make(map[storj.NodeID]*overlay.NodeReputation)
	for rows.Next() {
		var node overlay.NodeReputation
		node.Address = &pb.NodeAddress{}

		var lastIPPort sql.NullString
		var noise noiseScanner
		err = rows.Scan(&node.LastNet, &node.ID, &node.Address.Address, &node.Reputation.Email, &lastIPPort, &noise.Proto, &noise.PublicKey, &node.Reputation.VettedAt, &node.Reputation.UnknownAuditSuspended, &node.Reputation.OfflineSuspended)
		if err != nil {
			return nil, err
		}
		if lastIPPort.Valid {
			node.LastIPPort = lastIPPort.String
		}
		node.Address.NoiseInfo = noise.Convert()

		nodes[node.ID] = &node
	}

	return nodes, Error.Wrap(rows.Err())
}

// KnownOffline filters a set of nodes to offline nodes.
func (cache *overlaycache) KnownOffline(ctx context.Context, criteria *overlay.NodeCriteria, nodeIDs storj.NodeIDList) (offlineNodes storj.NodeIDList, err error) {
	for {
		offlineNodes, err = cache.knownOffline(ctx, criteria, nodeIDs)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return offlineNodes, err
		}
		break
	}

	return offlineNodes, err
}

func (cache *overlaycache) knownOffline(ctx context.Context, criteria *overlay.NodeCriteria, nodeIds storj.NodeIDList) (offlineNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIds) == 0 {
		return nil, Error.New("no ids provided")
	}

	// get offline nodes
	var rows tagsql.Rows
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id FROM nodes
		`+cache.db.impl.AsOfSystemInterval(criteria.AsOfSystemInterval)+`
			WHERE id = any($1::bytea[])
			AND last_contact_success < $2
		`), pgutil.NodeIDArray(nodeIds), time.Now().Add(-criteria.OnlineWindow),
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
	return offlineNodes, Error.Wrap(rows.Err())
}

// KnownUnreliableOrOffline filters a set of nodes to unreliable or offlines node, independent of new.
func (cache *overlaycache) KnownUnreliableOrOffline(ctx context.Context, criteria *overlay.NodeCriteria, nodeIDs storj.NodeIDList) (badNodes storj.NodeIDList, err error) {
	for {
		badNodes, err = cache.knownUnreliableOrOffline(ctx, criteria, nodeIDs)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return badNodes, err
		}
		break
	}

	return badNodes, err
}

// GetOfflineNodesForEmail gets nodes that we want to send an email to. These are non-disqualified, non-exited nodes where
// last_contact_success is between two points: the point where it is considered offline (offlineWindow), and the point where we don't want
// to send more emails (cutoff). It also filters nodes where last_offline_email is too recent (cooldown).
func (cache *overlaycache) GetOfflineNodesForEmail(ctx context.Context, offlineWindow, cutoff, cooldown time.Duration, limit int) (nodes map[storj.NodeID]string, err error) {
	defer mon.Task()(&ctx)(&err)

	now := time.Now()
	nodes = make(map[storj.NodeID]string)
	rows, err := cache.db.QueryContext(ctx, `
		SELECT id, email
		FROM nodes
		WHERE last_contact_success < $1
			AND last_contact_success > $2
			AND (last_offline_email IS NULL OR last_offline_email < $3)
			AND email != ''
			AND disqualified is NULL
			AND exit_finished_at is NULL
		LIMIT $4
	`, now.Add(-offlineWindow), now.Add(-cutoff), now.Add(-cooldown), limit)
	if err != nil {
		return
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var idBytes []byte
	var email string
	var nodeID storj.NodeID
	for rows.Next() {
		err = rows.Scan(&idBytes, &email)
		nodeID, err = storj.NodeIDFromBytes(idBytes)
		if err != nil {
			return
		}
		nodes[nodeID] = email
	}

	return nodes, Error.Wrap(rows.Err())
}

// UpdateLastOfflineEmail updates last_offline_email for a list of nodes.
func (cache *overlaycache) UpdateLastOfflineEmail(ctx context.Context, nodeIDs storj.NodeIDList, timestamp time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = cache.db.ExecContext(ctx, `
		UPDATE nodes
		SET last_offline_email = $1
		WHERE id = any($2::bytea[])
	`, timestamp, pgutil.NodeIDArray(nodeIDs))
	return err
}

// KnownReliableInExcludedCountries filters healthy nodes that are in excluded countries.
func (cache *overlaycache) KnownReliableInExcludedCountries(ctx context.Context, criteria *overlay.NodeCriteria, nodeIDs storj.NodeIDList) (reliableInExcluded storj.NodeIDList, err error) {
	for {
		reliableInExcluded, err = cache.knownReliableInExcludedCountries(ctx, criteria, nodeIDs)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return reliableInExcluded, err
		}
		break
	}

	return reliableInExcluded, err
}

func (cache *overlaycache) knownReliableInExcludedCountries(ctx context.Context, criteria *overlay.NodeCriteria, nodeIDs storj.NodeIDList) (reliableInExcluded storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIDs) == 0 {
		return nil, Error.New("no ids provided")
	}

	args := []interface{}{
		pgutil.NodeIDArray(nodeIDs),
		time.Now().Add(-criteria.OnlineWindow),
	}

	// When this config is not set, it's a string slice with one empty string. This is a sanity check just
	// in case for some reason it's nil or has no elements.
	if criteria.ExcludedCountries == nil || len(criteria.ExcludedCountries) == 0 {
		return reliableInExcluded, nil
	}

	var excludedCountriesCondition string
	if criteria.ExcludedCountries[0] == "" {
		return reliableInExcluded, nil
	}

	excludedCountriesCondition = "AND country_code IN (SELECT UNNEST($3::TEXT[]))"
	args = append(args, pgutil.TextArray(criteria.ExcludedCountries))

	// get reliable and online nodes
	var rows tagsql.Rows
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
			SELECT id
			FROM nodes
			`+cache.db.impl.AsOfSystemInterval(criteria.AsOfSystemInterval)+`
			WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND unknown_audit_suspended IS NULL
			AND offline_suspended IS NULL
			AND exit_finished_at IS  NULL
			AND last_contact_success > $2
			`+excludedCountriesCondition+`
		`), args...,
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
		reliableInExcluded = append(reliableInExcluded, id)
	}

	return reliableInExcluded, Error.Wrap(rows.Err())
}

func (cache *overlaycache) knownUnreliableOrOffline(ctx context.Context, criteria *overlay.NodeCriteria, nodeIDs storj.NodeIDList) (badNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIDs) == 0 {
		return nil, Error.New("no ids provided")
	}

	// get reliable and online nodes
	var rows tagsql.Rows
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
			SELECT id
			FROM nodes
			`+cache.db.impl.AsOfSystemInterval(criteria.AsOfSystemInterval)+`
			WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND unknown_audit_suspended IS NULL
			AND offline_suspended IS NULL
			AND exit_finished_at IS NULL
			AND last_contact_success > $2
		`), pgutil.NodeIDArray(nodeIDs), time.Now().Add(-criteria.OnlineWindow),
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	goodNodes := make(map[storj.NodeID]struct{}, len(nodeIDs))
	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		goodNodes[id] = struct{}{}
	}
	for _, id := range nodeIDs {
		if _, ok := goodNodes[id]; !ok {
			badNodes = append(badNodes, id)
		}
	}
	return badNodes, Error.Wrap(rows.Err())
}

// KnownReliable filters a set of nodes to reliable (online and qualified) nodes.
func (cache *overlaycache) KnownReliable(ctx context.Context, onlineWindow time.Duration, nodeIDs storj.NodeIDList) (nodes []*pb.Node, err error) {
	for {
		nodes, err = cache.knownReliable(ctx, onlineWindow, nodeIDs)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nodes, err
		}
		break
	}

	return nodes, err
}

func (cache *overlaycache) knownReliable(ctx context.Context, onlineWindow time.Duration, nodeIDs storj.NodeIDList) (nodes []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIDs) == 0 {
		return nil, Error.New("no ids provided")
	}

	// get online nodes
	rows, err := cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id, last_net, last_ip_port, address, protocol, noise_proto, noise_public_key
			FROM nodes
			WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND unknown_audit_suspended IS NULL
			AND offline_suspended IS NULL
			AND exit_finished_at IS NULL
			AND last_contact_success > $2
		`), pgutil.NodeIDArray(nodeIDs), time.Now().Add(-onlineWindow),
	)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		row := &dbx.Node{}
		err = rows.Scan(&row.Id, &row.LastNet, &row.LastIpPort, &row.Address, &row.Protocol, &row.NoiseProto, &row.NoisePublicKey)
		if err != nil {
			return nil, err
		}
		node, err := convertDBNode(ctx, row)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &node.Node)
	}
	return nodes, Error.Wrap(rows.Err())
}

// Reliable returns all reliable nodes.
func (cache *overlaycache) Reliable(ctx context.Context, criteria *overlay.NodeCriteria) (nodes storj.NodeIDList, err error) {
	for {
		nodes, err = cache.reliable(ctx, criteria)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nodes, err
		}
		break
	}

	return nodes, err
}

func (cache *overlaycache) reliable(ctx context.Context, criteria *overlay.NodeCriteria) (nodes storj.NodeIDList, err error) {
	args := []interface{}{
		time.Now().Add(-criteria.OnlineWindow),
	}

	// When this config is not set, it's a string slice with one empty string. I added some sanity checks to make sure we don't
	// dereference a nil pointer or index an element that doesn't exist.
	var excludedCountriesCondition string
	if criteria.ExcludedCountries != nil && len(criteria.ExcludedCountries) != 0 && criteria.ExcludedCountries[0] != "" {
		excludedCountriesCondition = "AND country_code NOT IN (SELECT UNNEST($2::TEXT[]))"
		args = append(args, pgutil.TextArray(criteria.ExcludedCountries))
	}

	// get reliable and online nodes
	rows, err := cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id
		FROM nodes
		`+cache.db.impl.AsOfSystemInterval(criteria.AsOfSystemInterval)+`
		WHERE disqualified IS NULL
		AND unknown_audit_suspended IS NULL
		AND offline_suspended IS NULL
		AND exit_finished_at IS NULL
		AND last_contact_success > $1
		`+excludedCountriesCondition+`
	`), args...)
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
	return nodes, Error.Wrap(rows.Err())
}

// UpdateReputation updates the DB columns for any of the reputation fields in ReputationUpdate.
func (cache *overlaycache) UpdateReputation(ctx context.Context, id storj.NodeID, request overlay.ReputationUpdate) (err error) {
	defer mon.Task()(&ctx)(&err)

	updateFields := dbx.Node_Update_Fields{}
	updateFields.UnknownAuditSuspended = dbx.Node_UnknownAuditSuspended_Raw(request.UnknownAuditSuspended)
	updateFields.OfflineSuspended = dbx.Node_OfflineSuspended_Raw(request.OfflineSuspended)
	updateFields.VettedAt = dbx.Node_VettedAt_Raw(request.VettedAt)

	updateFields.Disqualified = dbx.Node_Disqualified_Raw(request.Disqualified)
	if request.Disqualified != nil {
		updateFields.DisqualificationReason = dbx.Node_DisqualificationReason(int(request.DisqualificationReason))
	}

	err = cache.db.UpdateNoReturn_Node_By_Id_And_Disqualified_Is_Null_And_ExitFinishedAt_Is_Null(ctx, dbx.Node_Id(id.Bytes()), updateFields)
	return Error.Wrap(err)
}

// UpdateNodeInfo updates the following fields for a given node ID:
// wallet, email for node operator, free disk, and version.
func (cache *overlaycache) UpdateNodeInfo(ctx context.Context, nodeID storj.NodeID, nodeInfo *overlay.InfoResponse) (stats *overlay.NodeDossier, err error) {
	defer mon.Task()(&ctx)(&err)

	var updateFields dbx.Node_Update_Fields
	if nodeInfo != nil {
		if nodeInfo.Type != pb.NodeType_INVALID {
			updateFields.Type = dbx.Node_Type(int(nodeInfo.Type))
		}
		if nodeInfo.Operator != nil {
			walletFeatures, err := encodeWalletFeatures(nodeInfo.Operator.GetWalletFeatures())
			if err != nil {
				return nil, Error.Wrap(err)
			}

			updateFields.Wallet = dbx.Node_Wallet(nodeInfo.Operator.GetWallet())
			updateFields.Email = dbx.Node_Email(nodeInfo.Operator.GetEmail())
			updateFields.WalletFeatures = dbx.Node_WalletFeatures(walletFeatures)
		}
		if nodeInfo.Capacity != nil {
			updateFields.FreeDisk = dbx.Node_FreeDisk(nodeInfo.Capacity.GetFreeDisk())
		}
		if nodeInfo.Version != nil {
			semVer, err := version.NewSemVer(nodeInfo.Version.GetVersion())
			if err != nil {
				return nil, errs.New("unable to convert version to semVer")
			}
			updateFields.Major = dbx.Node_Major(int64(semVer.Major))
			updateFields.Minor = dbx.Node_Minor(int64(semVer.Minor))
			updateFields.Patch = dbx.Node_Patch(int64(semVer.Patch))
			updateFields.Hash = dbx.Node_Hash(nodeInfo.Version.GetCommitHash())
			updateFields.Timestamp = dbx.Node_Timestamp(nodeInfo.Version.Timestamp)
			updateFields.Release = dbx.Node_Release(nodeInfo.Version.GetRelease())
		}
	}

	updatedDBNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return convertDBNode(ctx, updatedDBNode)
}

// DisqualifyNode disqualifies a storage node.
func (cache *overlaycache) DisqualifyNode(ctx context.Context, nodeID storj.NodeID, disqualifiedAt time.Time, reason overlay.DisqualificationReason) (email string, err error) {
	defer mon.Task()(&ctx)(&err)
	updateFields := dbx.Node_Update_Fields{}
	updateFields.Disqualified = dbx.Node_Disqualified(disqualifiedAt.UTC())
	updateFields.DisqualificationReason = dbx.Node_DisqualificationReason(int(reason))

	dbNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return "", err
	}
	if dbNode == nil {
		return "", errs.New("unable to get node by ID: %v", nodeID)
	}
	return dbNode.Email, nil
}

// TestSuspendNodeUnknownAudit suspends a storage node for unknown audits.
func (cache *overlaycache) TestSuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	updateFields := dbx.Node_Update_Fields{}
	updateFields.UnknownAuditSuspended = dbx.Node_UnknownAuditSuspended(suspendedAt.UTC())

	dbNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return err
	}
	if dbNode == nil {
		return errs.New("unable to get node by ID: %v", nodeID)
	}
	return nil
}

// TestUnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
func (cache *overlaycache) TestUnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	updateFields := dbx.Node_Update_Fields{}
	updateFields.UnknownAuditSuspended = dbx.Node_UnknownAuditSuspended_Null()

	dbNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return err
	}
	if dbNode == nil {
		return errs.New("unable to get node by ID: %v", nodeID)
	}
	return nil
}

// AllPieceCounts returns a map of node IDs to piece counts from the db.
// NB: a valid, partial piece map can be returned even if node ID parsing error(s) are returned.
func (cache *overlaycache) AllPieceCounts(ctx context.Context) (_ map[storj.NodeID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	// NB: `All_Node_Id_Node_PieceCount_By_PieceCount_Not_Number` selects node
	// ID and piece count from the nodes table where piece count is not zero.
	rows, err := cache.db.All_Node_Id_Node_PieceCount_By_PieceCount_Not_Number(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pieceCounts := make(map[storj.NodeID]int64)
	nodeIDErrs := errs.Group{}
	for _, row := range rows {
		nodeID, err := storj.NodeIDFromBytes(row.Id)
		if err != nil {
			nodeIDErrs.Add(err)
			continue
		}
		pieceCounts[nodeID] = row.PieceCount
	}

	return pieceCounts, nodeIDErrs.Err()
}

func (cache *overlaycache) UpdatePieceCounts(ctx context.Context, pieceCounts map[storj.NodeID]int64) (err error) {
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
			Count: count,
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
	`, pgutil.NodeIDArray(nodeIDs), pgutil.Int8Array(countNumbers))

	return Error.Wrap(err)
}

// GetExitingNodes returns nodes who have initiated a graceful exit and is not disqualified, but have not completed it.
func (cache *overlaycache) GetExitingNodes(ctx context.Context) (exitingNodes []*overlay.ExitStatus, err error) {
	for {
		exitingNodes, err = cache.getExitingNodes(ctx)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return exitingNodes, err
		}
		break
	}

	return exitingNodes, err
}

func (cache *overlaycache) getExitingNodes(ctx context.Context) (exitingNodes []*overlay.ExitStatus, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id, exit_initiated_at, exit_loop_completed_at, exit_finished_at, exit_success FROM nodes
		WHERE exit_initiated_at IS NOT NULL
		AND exit_finished_at IS NULL
		AND disqualified is NULL
	`))
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var exitingNodeStatus overlay.ExitStatus
		err = rows.Scan(&exitingNodeStatus.NodeID, &exitingNodeStatus.ExitInitiatedAt, &exitingNodeStatus.ExitLoopCompletedAt, &exitingNodeStatus.ExitFinishedAt, &exitingNodeStatus.ExitSuccess)
		if err != nil {
			return nil, err
		}
		exitingNodes = append(exitingNodes, &exitingNodeStatus)
	}
	return exitingNodes, Error.Wrap(rows.Err())
}

// GetExitStatus returns a node's graceful exit status.
func (cache *overlaycache) GetExitStatus(ctx context.Context, nodeID storj.NodeID) (exitStatus *overlay.ExitStatus, err error) {
	for {
		exitStatus, err = cache.getExitStatus(ctx, nodeID)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return exitStatus, err
		}
		break
	}

	return exitStatus, err
}

func (cache *overlaycache) getExitStatus(ctx context.Context, nodeID storj.NodeID) (_ *overlay.ExitStatus, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id, exit_initiated_at, exit_loop_completed_at, exit_finished_at, exit_success
		FROM nodes
		WHERE id = ?
	`), nodeID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	exitStatus := &overlay.ExitStatus{}
	if rows.Next() {
		err = rows.Scan(&exitStatus.NodeID, &exitStatus.ExitInitiatedAt, &exitStatus.ExitLoopCompletedAt, &exitStatus.ExitFinishedAt, &exitStatus.ExitSuccess)
		if err != nil {
			return nil, err
		}
	}

	return exitStatus, Error.Wrap(rows.Err())
}

// GetGracefulExitCompletedByTimeFrame returns nodes who have completed graceful exit within a time window (time window is around graceful exit completion).
func (cache *overlaycache) GetGracefulExitCompletedByTimeFrame(ctx context.Context, begin, end time.Time) (exitedNodes storj.NodeIDList, err error) {
	for {
		exitedNodes, err = cache.getGracefulExitCompletedByTimeFrame(ctx, begin, end)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return exitedNodes, err
		}
		break
	}

	return exitedNodes, err
}

func (cache *overlaycache) getGracefulExitCompletedByTimeFrame(ctx context.Context, begin, end time.Time) (exitedNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id FROM nodes
		WHERE exit_initiated_at IS NOT NULL
		AND exit_finished_at IS NOT NULL
		AND exit_finished_at >= ?
		AND exit_finished_at < ?
	`), begin, end)
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
	return exitedNodes, Error.Wrap(rows.Err())
}

// GetGracefulExitIncompleteByTimeFrame returns nodes who have initiated, but not completed graceful exit within a time window (time window is around graceful exit initiation).
func (cache *overlaycache) GetGracefulExitIncompleteByTimeFrame(ctx context.Context, begin, end time.Time) (exitingNodes storj.NodeIDList, err error) {
	for {
		exitingNodes, err = cache.getGracefulExitIncompleteByTimeFrame(ctx, begin, end)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return exitingNodes, err
		}
		break
	}

	return exitingNodes, err
}

func (cache *overlaycache) getGracefulExitIncompleteByTimeFrame(ctx context.Context, begin, end time.Time) (exitingNodes storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id FROM nodes
		WHERE exit_initiated_at IS NOT NULL
		AND exit_finished_at IS NULL
		AND exit_initiated_at >= ?
		AND exit_initiated_at < ?
	`), begin, end)
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
	return exitingNodes, Error.Wrap(rows.Err())
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

	var noiseInfo *pb.NoiseInfo
	if info.NoiseProto != nil && len(info.NoisePublicKey) > 0 {
		noiseInfo = &pb.NoiseInfo{
			Proto:     pb.NoiseProtocol(*info.NoiseProto),
			PublicKey: info.NoisePublicKey,
		}
	}

	exitStatus := overlay.ExitStatus{NodeID: id}
	exitStatus.ExitInitiatedAt = info.ExitInitiatedAt
	exitStatus.ExitLoopCompletedAt = info.ExitLoopCompletedAt
	exitStatus.ExitFinishedAt = info.ExitFinishedAt
	exitStatus.ExitSuccess = info.ExitSuccess

	node := &overlay.NodeDossier{
		Node: pb.Node{
			Id: id,
			Address: &pb.NodeAddress{
				Address:   info.Address,
				NoiseInfo: noiseInfo,
			},
		},
		Type: pb.NodeType(info.Type),
		Operator: pb.NodeOperator{
			Email:          info.Email,
			Wallet:         info.Wallet,
			WalletFeatures: decodeWalletFeatures(info.WalletFeatures),
		},
		Capacity: pb.NodeCapacity{
			FreeDisk: info.FreeDisk,
		},
		Reputation: *getNodeStats(info),
		Version: pb.NodeVersion{
			Version:    ver.String(),
			CommitHash: info.Hash,
			Timestamp:  info.Timestamp,
			Release:    info.Release,
		},
		Disqualified:            info.Disqualified,
		DisqualificationReason:  (*overlay.DisqualificationReason)(info.DisqualificationReason),
		UnknownAuditSuspended:   info.UnknownAuditSuspended,
		OfflineSuspended:        info.OfflineSuspended,
		OfflineUnderReview:      info.UnderReview,
		PieceCount:              info.PieceCount,
		ExitStatus:              exitStatus,
		CreatedAt:               info.CreatedAt,
		LastNet:                 info.LastNet,
		LastOfflineEmail:        info.LastOfflineEmail,
		LastSoftwareUpdateEmail: info.LastSoftwareUpdateEmail,
	}
	if info.LastIpPort != nil {
		node.LastIPPort = *info.LastIpPort
	}
	if info.CountryCode != nil {
		node.CountryCode = location.ToCountryCode(*info.CountryCode)
	}
	if info.Contained != nil {
		node.Contained = true
	}

	return node, nil
}

// encodeWalletFeatures encodes wallet features into comma separated list string.
func encodeWalletFeatures(features []string) (string, error) {
	var errGroup errs.Group

	for _, feature := range features {
		if strings.Contains(feature, ",") {
			errGroup.Add(errs.New("error encoding %s, can not contain separator \",\"", feature))
		}
	}
	if err := errGroup.Err(); err != nil {
		return "", Error.Wrap(err)
	}

	return strings.Join(features, ","), nil
}

// decodeWalletFeatures decodes comma separated wallet features list string.
func decodeWalletFeatures(encoded string) []string {
	if encoded == "" {
		return nil
	}

	return strings.Split(encoded, ",")
}

func getNodeStats(dbNode *dbx.Node) *overlay.NodeStats {
	nodeStats := &overlay.NodeStats{
		Latency90:          dbNode.Latency90,
		LastContactSuccess: dbNode.LastContactSuccess,
		LastContactFailure: dbNode.LastContactFailure,
		OfflineUnderReview: dbNode.UnderReview,
		Status: overlay.ReputationStatus{
			Email:                 dbNode.Email,
			VettedAt:              dbNode.VettedAt,
			Disqualified:          dbNode.Disqualified,
			UnknownAuditSuspended: dbNode.UnknownAuditSuspended,
			OfflineSuspended:      dbNode.OfflineSuspended,
		},
	}
	return nodeStats
}

// DQNodesLastSeenBefore disqualifies a limited number of nodes where last_contact_success < cutoff except those already disqualified
// or gracefully exited or where last_contact_success = '0001-01-01 00:00:00+00'.
func (cache *overlaycache) DQNodesLastSeenBefore(ctx context.Context, cutoff time.Time, limit int) (nodeEmails map[storj.NodeID]string, count int, err error) {
	defer mon.Task()(&ctx)(&err)

	var nodeIDs []storj.NodeID
	nodeEmails = make(map[storj.NodeID]string)
	for {
		nodeIDs, err = cache.getNodesForDQLastSeenBefore(ctx, cutoff, limit)
		if err != nil {
			if cockroachutil.NeedsRetry(err) {
				continue
			}
			return nil, 0, err
		}
		if len(nodeIDs) == 0 {
			return nil, 0, nil
		}
		break
	}

	var rows tagsql.Rows
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
		UPDATE nodes
		SET disqualified = current_timestamp,
            disqualification_reason = $3
		WHERE id = any($1::bytea[])
			AND disqualified IS NULL
			AND exit_finished_at IS NULL
			AND last_contact_success < $2
			AND last_contact_success != '0001-01-01 00:00:00+00'::timestamptz
		RETURNING id, email, last_contact_success;
	`), pgutil.NodeIDArray(nodeIDs), cutoff, overlay.DisqualificationReasonNodeOffline)
	if err != nil {
		return nil, 0, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var id storj.NodeID
		var email string
		var lcs time.Time
		err = rows.Scan(&id, &email, &lcs)
		if err != nil {
			return nil, count, err
		}
		cache.db.log.Info("Disqualified",
			zap.String("DQ type", "stray node"),
			zap.Stringer("Node ID", id),
			zap.Stringer("Last contacted", lcs))
		nodeEmails[id] = email
		count++
	}
	return nodeEmails, count, rows.Err()
}

func (cache *overlaycache) getNodesForDQLastSeenBefore(ctx context.Context, cutoff time.Time, limit int) (nodes []storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := cache.db.Query(ctx, cache.db.Rebind(`
		SELECT id
		FROM nodes
		WHERE last_contact_success < $1
			AND disqualified is NULL
			AND exit_finished_at is NULL
			AND last_contact_success != '0001-01-01 00:00:00+00'::timestamptz
		LIMIT $2
	`), cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var nodeIDs []storj.NodeID
	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		nodeIDs = append(nodeIDs, id)
	}
	return nodeIDs, rows.Err()
}

func (cache *overlaycache) updateCheckInDirectUpdate(ctx context.Context, node overlay.NodeCheckInInfo, timestamp time.Time, semVer version.SemVer, walletFeatures string) (updated bool, err error) {
	var noiseProto sql.NullInt32
	var noisePublicKey []byte
	if node.Address.NoiseInfo != nil {
		noiseProto = sql.NullInt32{
			Int32: int32(node.Address.NoiseInfo.Proto),
			Valid: true,
		}
		noisePublicKey = node.Address.NoiseInfo.PublicKey
	}

	// First try the fast path.
	var res sql.Result
	res, err = cache.db.ExecContext(ctx, `
		UPDATE nodes
		SET
			address=$2,
			last_net=$3,
			protocol=$4,
			email=$5,
			wallet=$6,
			free_disk=$7,
			major=$9, minor=$10, patch=$11, hash=$12, timestamp=$13, release=$14,
			last_contact_success = CASE WHEN $8::bool IS TRUE
				THEN $15::timestamptz
				ELSE nodes.last_contact_success
			END,
			last_contact_failure = CASE WHEN $8::bool IS FALSE
				THEN $15::timestamptz
				ELSE nodes.last_contact_failure
			END,
			last_ip_port=$16,
			wallet_features=$17,
			country_code=$18,
			noise_proto=$21,
			noise_public_key=$22,
			last_software_update_email = CASE
				WHEN $19::bool IS TRUE THEN $15::timestamptz
				WHEN $20::bool IS FALSE THEN NULL
				ELSE nodes.last_software_update_email
			END,
			last_offline_email = CASE WHEN $8::bool IS TRUE
				THEN NULL
				ELSE nodes.last_offline_email
			END
		WHERE id = $1
	`, // args $1 - $4
		node.NodeID.Bytes(), node.Address.GetAddress(), node.LastNet, pb.NodeTransport_TCP_TLS_RPC,
		// args $5 - $7
		node.Operator.GetEmail(), node.Operator.GetWallet(), node.Capacity.GetFreeDisk(),
		// args $8
		node.IsUp,
		// args $9 - $14
		semVer.Major, semVer.Minor, semVer.Patch, node.Version.GetCommitHash(), node.Version.Timestamp, node.Version.GetRelease(),
		// args $15
		timestamp,
		// args $16
		node.LastIPPort,
		// args $17,
		walletFeatures,
		// args $18,
		node.CountryCode.String(),
		// args $19 - $20
		node.SoftwareUpdateEmailSent, node.VersionBelowMin,
		// args $21 - $22
		noiseProto, noisePublicKey,
	)

	if err != nil {
		return false, Error.Wrap(err)
	}
	affected, affectedErr := res.RowsAffected()
	if affectedErr != nil {
		return false, Error.Wrap(err)
	}
	return affected > 0, nil
}

// UpdateCheckIn updates a single storagenode with info from when the the node last checked in.
func (cache *overlaycache) UpdateCheckIn(ctx context.Context, node overlay.NodeCheckInInfo, timestamp time.Time, config overlay.NodeSelectionConfig) (err error) {
	defer mon.Task()(&ctx)(&err)

	if node.Address.GetAddress() == "" {
		return Error.New("error UpdateCheckIn: missing the storage node address")
	}

	semVer, err := version.NewSemVer(node.Version.GetVersion())
	if err != nil {
		return Error.New("unable to convert version to semVer")
	}

	walletFeatures, err := encodeWalletFeatures(node.Operator.GetWalletFeatures())
	if err != nil {
		return Error.Wrap(err)
	}

	updated, err := cache.updateCheckInDirectUpdate(ctx, node, timestamp, semVer, walletFeatures)
	if err != nil {
		return Error.Wrap(err)
	}
	if updated {
		return nil
	}

	var noiseProto sql.NullInt32
	var noisePublicKey []byte
	if node.Address.NoiseInfo != nil {
		noiseProto = sql.NullInt32{
			Int32: int32(node.Address.NoiseInfo.Proto),
			Valid: true,
		}
		noisePublicKey = node.Address.NoiseInfo.PublicKey
	}

	_, err = cache.db.ExecContext(ctx, `
			INSERT INTO nodes
			(
				id, address, last_net, protocol, type,
				email, wallet, free_disk,
				last_contact_success,
				last_contact_failure,
				major, minor, patch, hash, timestamp, release,
				last_ip_port, wallet_features, country_code,
				noise_proto, noise_public_key
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8,
				CASE WHEN $9::bool IS TRUE THEN $16::timestamptz
					ELSE '0001-01-01 00:00:00+00'::timestamptz
				END,
				CASE WHEN $9::bool IS FALSE THEN $16::timestamptz
					ELSE '0001-01-01 00:00:00+00'::timestamptz
				END,
				$10, $11, $12, $13, $14, $15,
				$17, $18, $19,
				$22, $23
			)
			ON CONFLICT (id)
			DO UPDATE
			SET
				address=$2,
				last_net=$3,
				protocol=$4,
				email=$6,
				wallet=$7,
				free_disk=$8,
				major=$10, minor=$11, patch=$12, hash=$13, timestamp=$14, release=$15,
				last_contact_success = CASE WHEN $9::bool IS TRUE
					THEN $16::timestamptz
					ELSE nodes.last_contact_success
				END,
				last_contact_failure = CASE WHEN $9::bool IS FALSE
					THEN $16::timestamptz
					ELSE nodes.last_contact_failure
				END,
				last_ip_port=$17,
				wallet_features=$18,
				country_code=$19,
				noise_proto=$22,
				noise_public_key=$23,
				last_software_update_email = CASE
					WHEN $20::bool IS TRUE THEN $16::timestamptz
					WHEN $21::bool IS FALSE THEN NULL
					ELSE nodes.last_software_update_email
				END,
				last_offline_email = CASE WHEN $9::bool IS TRUE
					THEN NULL
					ELSE nodes.last_offline_email
				END;
			`,
		// args $1 - $5
		node.NodeID.Bytes(), node.Address.GetAddress(), node.LastNet, pb.NodeTransport_TCP_TLS_RPC, int(pb.NodeType_STORAGE),
		// args $6 - $8
		node.Operator.GetEmail(), node.Operator.GetWallet(), node.Capacity.GetFreeDisk(),
		// args $9
		node.IsUp,
		// args $10 - $15
		semVer.Major, semVer.Minor, semVer.Patch, node.Version.GetCommitHash(), node.Version.Timestamp, node.Version.GetRelease(),
		// args $16
		timestamp,
		// args $17 - $19
		node.LastIPPort, walletFeatures, node.CountryCode.String(),
		// args $20 - $21
		node.SoftwareUpdateEmailSent, node.VersionBelowMin,
		// args $22 - $23
		noiseProto, noisePublicKey,
	)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// SetNodeContained updates the contained field for the node record. If
// `contained` is true, the contained field in the record is set to the current
// database time, if it is not already set. If `contained` is false, the
// contained field in the record is set to NULL. All other fields are left
// alone.
func (cache *overlaycache) SetNodeContained(ctx context.Context, nodeID storj.NodeID, contained bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	var query string
	if contained {
		// only update the timestamp if it's not already set
		query = `
			UPDATE nodes SET contained = current_timestamp
			WHERE id = $1 AND contained IS NULL
		`
	} else {
		query = `
			UPDATE nodes SET contained = NULL
			WHERE id = $1
		`
	}
	_, err = cache.db.DB.ExecContext(ctx, query, nodeID[:])
	return Error.Wrap(err)
}

// SetAllContainedNodes updates the contained field for all nodes, as necessary.
// containedNodes is expected to be a set of all nodes that should be contained.
// All nodes which are in this set but do not already have a non-NULL contained
// field will be updated to be contained as of the current time, and all nodes
// which are not in this set but are contained in the table will be updated to
// have a NULL contained field.
func (cache *overlaycache) SetAllContainedNodes(ctx context.Context, containedNodes []storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	updateQuery := `
		WITH should_be AS (
			SELECT nodes.id, EXISTS (SELECT 1 FROM unnest($1::BYTEA[]) sb(i) WHERE sb.i = id) AS contained
			FROM nodes
		)
		UPDATE nodes n SET contained =
		    CASE WHEN should_be.contained
		        THEN current_timestamp
		        ELSE NULL
		    END
		FROM should_be
		WHERE n.id = should_be.id
			AND (n.contained IS NOT NULL) != should_be.contained
	`
	_, err = cache.db.DB.ExecContext(ctx, updateQuery, pgutil.NodeIDArray(containedNodes))
	return Error.Wrap(err)
}

var (
	// ErrVetting is the error class for the following test methods.
	ErrVetting = errs.Class("vetting")
)

// TestVetNode directly sets a node's vetted_at timestamp to make testing easier.
func (cache *overlaycache) TestVetNode(ctx context.Context, nodeID storj.NodeID) (vettedTime *time.Time, err error) {
	updateFields := dbx.Node_Update_Fields{
		VettedAt: dbx.Node_VettedAt(time.Now().UTC()),
	}
	node, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return nil, err
	}
	return node.VettedAt, nil
}

// TestUnvetNode directly sets a node's vetted_at timestamp to null to make testing easier.
func (cache *overlaycache) TestUnvetNode(ctx context.Context, nodeID storj.NodeID) (err error) {
	_, err = cache.db.Exec(ctx, `UPDATE nodes SET vetted_at = NULL WHERE nodes.id = $1;`, nodeID)
	if err != nil {
		return err
	}
	_, err = cache.Get(ctx, nodeID)
	return err
}

// TestSuspendNodeOffline suspends a storage node for offline.
func (cache *overlaycache) TestSuspendNodeOffline(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)
	updateFields := dbx.Node_Update_Fields{}
	updateFields.OfflineSuspended = dbx.Node_OfflineSuspended(suspendedAt.UTC())

	dbNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return err
	}
	if dbNode == nil {
		return errs.New("unable to get node by ID: %v", nodeID)
	}
	return nil
}

// TestNodeCountryCode sets node country code.
func (cache *overlaycache) TestNodeCountryCode(ctx context.Context, nodeID storj.NodeID, countryCode string) (err error) {
	defer mon.Task()(&ctx)(&err)
	updateFields := dbx.Node_Update_Fields{}
	updateFields.CountryCode = dbx.Node_CountryCode(countryCode)

	dbNode, err := cache.db.Update_Node_By_Id(ctx, dbx.Node_Id(nodeID.Bytes()), updateFields)
	if err != nil {
		return err
	}
	if dbNode == nil {
		return errs.New("unable to set node country code: %v", nodeID)
	}
	return nil
}

// IterateAllContactedNodes will call cb on all known nodes (used in restore trash contexts).
func (cache *overlaycache) IterateAllContactedNodes(ctx context.Context, cb func(context.Context, *overlay.SelectedNode) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	var rows tagsql.Rows
	// 2018-04-06 is the date of the first storj v3 commit.
	rows, err = cache.db.Query(ctx, cache.db.Rebind(`
		SELECT last_net, id, address, last_ip_port, noise_proto, noise_public_key
		FROM nodes
		WHERE last_contact_success >= timestamp '2018-04-06'
	`))
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var node overlay.SelectedNode
		node.Address = &pb.NodeAddress{}

		var lastIPPort sql.NullString
		var noise noiseScanner
		err = rows.Scan(&node.LastNet, &node.ID, &node.Address.Address, &lastIPPort, &noise.Proto, &noise.PublicKey)
		if err != nil {
			return Error.Wrap(err)
		}
		if lastIPPort.Valid {
			node.LastIPPort = lastIPPort.String
		}
		node.Address.NoiseInfo = noise.Convert()

		err = cb(ctx, &node)
		if err != nil {
			return err
		}
	}

	return rows.Err()
}

// IterateAllNodeDossiers will call cb on all known nodes (used for invoice generation).
func (cache *overlaycache) IterateAllNodeDossiers(ctx context.Context, cb func(context.Context, *overlay.NodeDossier) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	const nodesPerPage = 1000
	var cont *dbx.Paged_Node_Continuation
	var dbxNodes []*dbx.Node

	for {
		dbxNodes, cont, err = cache.db.Paged_Node(ctx, nodesPerPage, cont)
		if err != nil {
			return err
		}

		for _, node := range dbxNodes {
			dossier, err := convertDBNode(ctx, node)
			if err != nil {
				return err
			}
			if err := cb(ctx, dossier); err != nil {
				return err
			}
		}

		if cont == nil {
			return nil
		}
	}
}

func (cache *overlaycache) TestUpdateCheckInDirectUpdate(ctx context.Context, node overlay.NodeCheckInInfo, timestamp time.Time, semVer version.SemVer, walletFeatures string) (updated bool, err error) {
	return cache.updateCheckInDirectUpdate(ctx, node, timestamp, semVer, walletFeatures)
}

type noiseScanner struct {
	Proto     sql.NullInt32
	PublicKey []byte
}

func (n *noiseScanner) Convert() *pb.NoiseInfo {
	if !n.Proto.Valid || len(n.PublicKey) == 0 {
		return nil
	}
	return &pb.NoiseInfo{
		Proto:     pb.NoiseProtocol(n.Proto.Int32),
		PublicKey: n.PublicKey,
	}
}

// OneTimeFixLastNets updates the last_net values for all node records to be equal to their
// last_ip_port values.
//
// This is only appropriate to do when the satellite has DistinctIP=false and has been upgraded from
// before changeset I0e7e92498c3da768df5b4d5fb213dcd2d4862924. It is only necessary to run this
// once, after all satellite peers are upgraded (otherwise some nodes may be inserted with truncated
// last_net values, and those nodes could be unfairly disadvantaged in node selection).
func (cache *overlaycache) OneTimeFixLastNets(ctx context.Context) error {
	_, err := cache.db.ExecContext(ctx, "UPDATE nodes SET last_net = last_ip_port")
	return err
}
