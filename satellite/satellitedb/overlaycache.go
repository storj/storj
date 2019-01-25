// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/utils"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

var _ overlay.DB = (*overlaycache)(nil)

type overlaycache struct {
	db *dbx.DB
}

type filterReputableRequest struct {
	minReputation         *pb.NodeStats
	restrictions          *pb.NodeRestrictions
	excluded              []pb.NodeID
	reputableNodeAmount   int64
	newNodeAuditThreshold int64
	base                  int
}

type filterNewRequest struct {
	restrictions          *pb.NodeRestrictions
	excluded              []pb.NodeID
	newNodeAmount         int64
	newNodeAuditThreshold int64
	base                  int
}

// FilterNodes looks up nodes based on reputation requirements
func (cache *overlaycache) FilterNodes(ctx context.Context, req *overlay.FilterNodesRequest) ([]*pb.Node, error) {
	filterReputableReq := &filterReputableRequest{
		minReputation:         req.MinReputation,
		restrictions:          req.Restrictions,
		excluded:              req.Excluded,
		reputableNodeAmount:   req.ReputableNodeAmount,
		newNodeAuditThreshold: req.NewNodeAuditThreshold,
		// todo: make base configurable
		base: 10,
	}

	reputableNodes, err := cache.filterReputableNodes(ctx, filterReputableReq)
	if err != nil {
		return nil, err
	}

	newNodeAmount := req.ReputableNodeAmount * int64(float64(req.ReputableNodeAmount)*req.NewNodePercentage)

	filterNewReq := &filterNewRequest{
		restrictions:          req.Restrictions,
		excluded:              req.Excluded,
		newNodeAmount:         newNodeAmount,
		newNodeAuditThreshold: req.NewNodeAuditThreshold,
		// todo: make base configurable
		base: 10,
	}

	newNodes, err := cache.filterNewNodes(ctx, filterNewReq)
	if err != nil {
		return nil, err
	}

	var allNodes []*pb.Node
	allNodes = append(allNodes, reputableNodes...)
	allNodes = append(allNodes, newNodes...)

	return allNodes, nil
}

func (cache *overlaycache) filterReputableNodes(ctx context.Context, req *filterReputableRequest) (reputableNodes []*pb.Node, err error) {
	rows, err := cache.findReputableNodesQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = utils.CombineErrors(err, rows.Close())
	}()

	for rows.Next() {
		node := &dbx.Node{}
		err = rows.Scan(&node.Id)
		if err != nil {
			return nil, err
		}
		id, err := storj.NodeIDFromBytes(node.Id)
		if err != nil {
			return nil, err
		}
		info, err := cache.Get(ctx, id)
		if err != nil {
			continue
		}
		reputableNodes = append(reputableNodes, info)
	}

	return reputableNodes, nil
}

func (cache *overlaycache) filterNewNodes(ctx context.Context, req *filterNewRequest) (newNodes []*pb.Node, err error) {
	rows, err := cache.findNewNodesQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = utils.CombineErrors(err, rows.Close())
	}()

	for rows.Next() {
		node := &dbx.Node{}
		err = rows.Scan(&node.Id)
		if err != nil {
			return nil, err
		}
		id, err := storj.NodeIDFromBytes(node.Id)
		if err != nil {
			return nil, err
		}
		info, err := cache.Get(ctx, id)
		if err != nil {
			continue
		}
		newNodes = append(newNodes, info)
	}

	return newNodes, nil
}

func (cache *overlaycache) findReputableNodesQuery(ctx context.Context, req *filterReputableRequest) (*sql.Rows, error) {
	var auditCount int64
	if req.minReputation.AuditCount > req.newNodeAuditThreshold {
		auditCount = req.minReputation.AuditCount
	} else {
		auditCount = req.newNodeAuditThreshold
	}

	// todo: add filtering for free bandwidth and free disk space and not in excluded
	rows, err := cache.db.Query(cache.db.Rebind(`SELECT nodes.id,
		FROM nodes
		LIMIT ` + strconv.FormatInt(req.reputableNodeAmount, req.base) + `
		WHERE nodes.total_audit_count >` + strconv.FormatInt(auditCount, req.base) + `
		AND nodes.audit_success_ratio >` + fmt.Sprintf("%v", req.minReputation.AuditSuccessRatio) + `
		AND nodes.total_uptime_count >` + strconv.FormatInt(req.minReputation.UptimeCount, req.base) + `
		AND nodes.uptime_ratio >` + fmt.Sprintf("%v", req.minReputation.UptimeRatio),
	))
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (cache *overlaycache) findNewNodesQuery(ctx context.Context, req *filterNewRequest) (*sql.Rows, error) {
	// todo: add filtering for free bandwidth and free disk space and not in excluded
	rows, err := cache.db.Query(cache.db.Rebind(`SELECT nodes.id,
		FROM nodes
		LIMIT ` + strconv.FormatInt(req.newNodeAmount, req.base) + `
		WHERE nodes.total_audit_count <` + strconv.FormatInt(req.newNodeAuditThreshold, req.base),
	))
	if err != nil {
		return nil, err
	}

	return rows, nil
}

// Get looks up the node by nodeID
func (cache *overlaycache) Get(ctx context.Context, id storj.NodeID) (*pb.Node, error) {
	if id.IsZero() {
		return nil, overlay.ErrEmptyNode
	}

	node, err := cache.db.Get_OverlayCacheNode_By_NodeId(ctx,
		dbx.OverlayCacheNode_NodeId(id.Bytes()),
	)
	if err == sql.ErrNoRows {
		return nil, overlay.ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}

	return convertOverlayNode(node)
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

	dbxInfos, err := cache.db.Limited_OverlayCacheNode_By_NodeId_GreaterOrEqual(ctx,
		dbx.OverlayCacheNode_NodeId(cursor.Bytes()),
		limit, 0,
	)
	if err != nil {
		return nil, err
	}

	infos := make([]*pb.Node, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i], err = convertOverlayNode(dbxInfo)
		if err != nil {
			return nil, err
		}
	}
	return infos, nil
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
	_, err = tx.Get_OverlayCacheNode_By_NodeId(ctx,
		dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),
	)

	address := info.Address
	if address == nil {
		address = &pb.NodeAddress{}
	}

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

	if err != nil {
		_, err = tx.Create_OverlayCacheNode(
			ctx,
			dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),

			dbx.OverlayCacheNode_NodeType(int(info.Type)),
			dbx.OverlayCacheNode_Address(address.Address),
			dbx.OverlayCacheNode_Protocol(int(address.Transport)),

			dbx.OverlayCacheNode_OperatorEmail(metadata.Email),
			dbx.OverlayCacheNode_OperatorWallet(metadata.Wallet),

			dbx.OverlayCacheNode_FreeBandwidth(restrictions.FreeBandwidth),
			dbx.OverlayCacheNode_FreeDisk(restrictions.FreeDisk),

			dbx.OverlayCacheNode_Latency90(reputation.Latency_90),
			dbx.OverlayCacheNode_AuditSuccessRatio(reputation.AuditSuccessRatio),
			dbx.OverlayCacheNode_AuditUptimeRatio(reputation.UptimeRatio),
			dbx.OverlayCacheNode_AuditCount(reputation.AuditCount),
			dbx.OverlayCacheNode_AuditSuccessCount(reputation.AuditSuccessCount),

			dbx.OverlayCacheNode_UptimeCount(reputation.UptimeCount),
			dbx.OverlayCacheNode_UptimeSuccessCount(reputation.UptimeSuccessCount),
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		update := dbx.OverlayCacheNode_Update_Fields{
			// TODO: should we be able to update node type?
			Address:  dbx.OverlayCacheNode_Address(address.Address),
			Protocol: dbx.OverlayCacheNode_Protocol(int(address.Transport)),

			Latency90:          dbx.OverlayCacheNode_Latency90(info.Reputation.Latency_90),
			AuditSuccessRatio:  dbx.OverlayCacheNode_AuditSuccessRatio(info.Reputation.AuditSuccessRatio),
			AuditUptimeRatio:   dbx.OverlayCacheNode_AuditUptimeRatio(info.Reputation.UptimeRatio),
			AuditCount:         dbx.OverlayCacheNode_AuditCount(info.Reputation.AuditCount),
			AuditSuccessCount:  dbx.OverlayCacheNode_AuditSuccessCount(info.Reputation.AuditSuccessCount),
			UptimeCount:        dbx.OverlayCacheNode_UptimeCount(info.Reputation.UptimeCount),
			UptimeSuccessCount: dbx.OverlayCacheNode_UptimeSuccessCount(info.Reputation.UptimeSuccessCount),
		}

		if info.Metadata != nil {
			update.OperatorEmail = dbx.OverlayCacheNode_OperatorEmail(info.Metadata.Email)
			update.OperatorWallet = dbx.OverlayCacheNode_OperatorWallet(info.Metadata.Wallet)
		}

		if info.Restrictions != nil {
			update.FreeBandwidth = dbx.OverlayCacheNode_FreeBandwidth(restrictions.FreeBandwidth)
			update.FreeDisk = dbx.OverlayCacheNode_FreeDisk(restrictions.FreeDisk)
		}

		_, err := tx.Update_OverlayCacheNode_By_NodeId(ctx,
			dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),
			update,
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}

	return Error.Wrap(tx.Commit())
}

// Delete deletes node based on id
func (cache *overlaycache) Delete(ctx context.Context, id storj.NodeID) error {
	_, err := cache.db.Delete_OverlayCacheNode_By_NodeId(ctx,
		dbx.OverlayCacheNode_NodeId(id.Bytes()),
	)
	return err
}

func convertOverlayNode(info *dbx.OverlayCacheNode) (*pb.Node, error) {
	if info == nil {
		return nil, Error.New("missing info")
	}

	id, err := storj.NodeIDFromBytes(info.NodeId)
	if err != nil {
		return nil, err
	}

	node := &pb.Node{
		Id:   id,
		Type: pb.NodeType(info.NodeType),
		Address: &pb.NodeAddress{
			Address:   info.Address,
			Transport: pb.NodeTransport(info.Protocol),
		},
		Metadata: &pb.NodeMetadata{
			Email:  info.OperatorEmail,
			Wallet: info.OperatorWallet,
		},
		Restrictions: &pb.NodeRestrictions{
			FreeBandwidth: info.FreeBandwidth,
			FreeDisk:      info.FreeDisk,
		},
		Reputation: &pb.NodeStats{
			NodeId:             id,
			Latency_90:         info.Latency90,
			AuditSuccessRatio:  info.AuditSuccessRatio,
			UptimeRatio:        info.AuditUptimeRatio,
			AuditCount:         info.AuditCount,
			AuditSuccessCount:  info.AuditSuccessCount,
			UptimeCount:        info.UptimeCount,
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

	return node, nil
}

//GetWalletAddress gets the node's wallet address
func (cache *overlaycache) GetWalletAddress(ctx context.Context, id storj.NodeID) (string, error) {
	w, err := cache.db.Get_OverlayCacheNode_OperatorWallet_By_NodeId(ctx, dbx.OverlayCacheNode_NodeId(id.Bytes()))
	if err != nil {
		return "", err
	}
	return w.OperatorWallet, nil
}
