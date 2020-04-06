// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/private/version"
	"storj.io/storj/satellite/overlay"
)

func (cache *overlaycache) SelectStorageNodes(ctx context.Context, reputableNodeCount, newNodeCount int, criteria *overlay.NodeCriteria) (nodes []*overlay.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)
	if newNodeCount == 0 && reputableNodeCount == 0 {
		return nil, nil
	}

	needReputableNodes := reputableNodeCount
	needNewNodes := newNodeCount

	receivedNodeNetworks := make(map[string]struct{})

	excludedIDs := append([]storj.NodeID{}, criteria.ExcludedIDs...)
	excludedNetworks := append([]string{}, criteria.ExcludedNetworks...)

	for i := 0; i < 3; i++ {
		reputableNodes, newNodes, err := cache.selectStorageNodesOnce(ctx, needReputableNodes, needNewNodes, criteria, excludedIDs, excludedNetworks)
		if err != nil {
			return nil, err
		}

		for _, node := range newNodes {
			// checking for last net collision among reputable and new nodes since we can't check within the query
			if _, ok := receivedNodeNetworks[node.LastNet]; ok {
				continue
			}

			excludedIDs = append(excludedIDs, node.ID)
			excludedNetworks = append(excludedNetworks, node.LastNet)

			needNewNodes--
			nodes = append(nodes, node)

			if criteria.DistinctIP {
				receivedNodeNetworks[node.LastNet] = struct{}{}
			}
		}

		for _, node := range reputableNodes {
			if _, ok := receivedNodeNetworks[node.LastNet]; ok {
				continue
			}

			excludedIDs = append(excludedIDs, node.ID)
			excludedNetworks = append(excludedNetworks, node.LastNet)

			needReputableNodes--
			nodes = append(nodes, node)

			if criteria.DistinctIP {
				receivedNodeNetworks[node.LastNet] = struct{}{}
			}
		}

		if needReputableNodes <= 0 && needNewNodes <= 0 {
			break
		}
	}

	return nodes, nil
}

func (cache *overlaycache) selectStorageNodesOnce(ctx context.Context, reputableNodeCount, newNodeCount int, criteria *overlay.NodeCriteria, excludedIDs []storj.NodeID, excludedNetworks []string) (reputableNodes, newNodes []*overlay.SelectedNode, err error) {
	defer mon.Task()(&ctx)(&err)

	newNodesCondition, err := nodeSelectionCondition(ctx, criteria, excludedIDs, excludedNetworks, true)
	if err != nil {
		return nil, nil, err
	}

	reputableNodesCondition, err := nodeSelectionCondition(ctx, criteria, excludedIDs, excludedNetworks, false)
	if err != nil {
		return nil, nil, err
	}

	var reputableNodeQuery, newNodeQuery partialQuery
	if !criteria.DistinctIP {
		reputableNodeQuery = partialQuery{
			selection: `SELECT last_net, id, address, last_ip_port, true FROM nodes`,
			condition: reputableNodesCondition,
			limit:     reputableNodeCount,
		}
		newNodeQuery = partialQuery{
			selection: `SELECT last_net, id, address, last_ip_port, false FROM nodes`,
			condition: newNodesCondition,
			limit:     newNodeCount,
		}
	} else {
		reputableNodeQuery = partialQuery{
			selection: `SELECT DISTINCT ON (last_net) last_net, id, address, last_ip_port, true FROM nodes`,
			condition: reputableNodesCondition,
			distinct:  true,
			limit:     reputableNodeCount,
			orderBy:   "last_net",
		}
		newNodeQuery = partialQuery{
			selection: `SELECT DISTINCT ON (last_net) last_net, id, address, last_ip_port, false FROM nodes`,
			condition: newNodesCondition,
			distinct:  true,
			limit:     newNodeCount,
			orderBy:   "last_net",
		}
	}

	query := unionAll(newNodeQuery, reputableNodeQuery)
	rows, err := cache.db.Query(ctx, cache.db.Rebind(query.query), query.args...)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var node overlay.SelectedNode
		node.Address = &pb.NodeAddress{Transport: pb.NodeTransport_TCP_TLS_GRPC}
		var lastIPPort sql.NullString
		var isNew bool

		err = rows.Scan(&node.LastNet, &node.ID, &node.Address.Address, &node.LastIPPort, &isNew)
		if err != nil {
			return nil, nil, err
		}
		if lastIPPort.Valid {
			node.LastIPPort = lastIPPort.String
		}

		if isNew {
			newNodes = append(newNodes, &node)
		} else {
			reputableNodes = append(reputableNodes, &node)
		}
	}

	return reputableNodes, newNodes, Error.Wrap(rows.Err())
}

func nodeSelectionCondition(ctx context.Context, criteria *overlay.NodeCriteria, excludedIDs []storj.NodeID, excludedNetworks []string, isNewNodeQuery bool) (condition, error) {
	var conds conditions

	conds.add("disqualified IS NULL")
	conds.add("suspended IS NULL")
	conds.add("exit_initiated_at IS NULL")
	conds.add("type = ?", int(pb.NodeType_STORAGE))

	conds.add("free_disk >= ?", criteria.FreeDisk)
	conds.add("last_contact_success > ?", time.Now().Add(-criteria.OnlineWindow))

	if isNewNodeQuery {
		conds.add(
			"total_audit_count < ?",
			criteria.AuditCount,
		)
	} else {
		conds.add(
			"total_audit_count >= ? AND total_uptime_count >= ?",
			criteria.AuditCount, criteria.UptimeCount,
		)
	}

	if criteria.MinimumVersion != "" {
		v, err := version.NewSemVer(criteria.MinimumVersion)
		if err != nil {
			return condition{}, Error.New("invalid node selection criteria version: %v", err)
		}
		conds.add(
			"(major > ? OR (major = ? AND (minor > ? OR (minor = ? AND patch >= ?)))) AND release",
			v.Major, v.Major, v.Minor, v.Minor, v.Patch,
		)
	}

	if len(excludedIDs) > 0 {
		conds.addWithIDs(
			" AND id NOT IN (?"+strings.Repeat(", ?", len(excludedIDs)-1)+")",
			excludedIDs...,
		)
	}

	if criteria.DistinctIP {
		if len(excludedNetworks) > 0 {
			conds.addWithStrings(
				" AND last_net NOT IN (?"+strings.Repeat(", ?", len(excludedIDs)-1)+") ",
				excludedNetworks...,
			)
		}
		conds.add("AND last_net <> ''")
	}

	return conds.combine(), nil
}

func unionAll(partials ...partialQuery) query {
	var queries []string
	var args []interface{}
	for _, partial := range partials {
		if partial.isEmpty() {
			continue
		}

		q := partial.asQuery()
		queries = append(queries, q.query)
		args = append(args, q.args...)
	}

	return query{
		query: "(" + strings.Join(queries, ") UNION ALL (") + ")",
		args:  args,
	}
}

type condition struct {
	query string
	args  []interface{}
}

type partialQuery struct {
	selection string
	condition condition
	distinct  bool
	orderBy   string
	limit     int
}

func (q partialQuery) isEmpty() bool {
	return q.limit == 0
}

func (partial partialQuery) asQuery() query {
	var q strings.Builder
	var args []interface{}

	if partial.distinct {
		fmt.Fprintf(&q, "SELECT * FROM (")
	}

	fmt.Fprint(&q, partial.selection)

	fmt.Fprintf(&q, " WHERE %s ", partial.condition.query)
	args = append(args, partial.condition.args...)

	if partial.orderBy != "" {
		fmt.Fprintf(&q, " ORDER BY %s, RANDOM() ", partial.orderBy)
	} else {
		fmt.Fprint(&q, " ORDER BY RANDOM() ")
	}

	fmt.Fprintf(&q, " LIMIT ? ")
	args = append(args, partial.limit)

	if partial.distinct {
		fmt.Fprintf(&q, ") ORDER BY RANDOM() LIMIT ?")
		args = append(args, partial.limit)
	}

	return query{
		query: q.String(),
		args:  args,
	}
}

type query struct {
	query string
	args  []interface{}
}

type conditions []condition

func (xs *conditions) add(q string, args ...interface{}) {
	*xs = append(*xs, cond(q, args...))
}

func (xs *conditions) addWithStrings(q string, args ...string) {
	var values []interface{}
	for _, arg := range args {
		values = append(values, arg)
	}
	*xs = append(*xs, condition{query: q, args: values})
}

func (xs *conditions) addWithIDs(q string, args ...storj.NodeID) {
	var values []interface{}
	for _, arg := range args {
		values = append(values, arg)
	}
	*xs = append(*xs, condition{query: q, args: values})
}

func (conditions conditions) combine() condition {
	var qs []string
	var args []interface{}
	for _, c := range conditions {
		qs = append(qs, c.query)
		args = append(args, c.args...)
	}
	return condition{query: strings.Join(qs, " AND "), args: args})
}
