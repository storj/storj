// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/overlay"
)

var (
	mon = monkit.Package()
)

// Endpoint for querying node stats for the SNO
//
// architecture: Endpoint
type Endpoint struct {
	log        *zap.Logger
	overlay    overlay.DB
	accounting accounting.StoragenodeAccounting
}

// NewEndpoint creates new endpoint
func NewEndpoint(log *zap.Logger, overlay overlay.DB, accounting accounting.StoragenodeAccounting) *Endpoint {
	return &Endpoint{
		log:        log,
		overlay:    overlay,
		accounting: accounting,
	}
}

// GetStats sends node stats for client node
func (e *Endpoint) GetStats(ctx context.Context, req *pb.GetStatsRequest) (_ *pb.GetStatsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
		}
		e.log.Error("overlay.Get failed", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	uptimeScore := calculateReputationScore(
		node.Reputation.UptimeReputationAlpha,
		node.Reputation.UptimeReputationBeta)

	auditScore := calculateReputationScore(
		node.Reputation.AuditReputationAlpha,
		node.Reputation.AuditReputationBeta)

	return &pb.GetStatsResponse{
		UptimeCheck: &pb.ReputationStats{
			TotalCount:      node.Reputation.UptimeCount,
			SuccessCount:    node.Reputation.UptimeSuccessCount,
			ReputationAlpha: node.Reputation.UptimeReputationAlpha,
			ReputationBeta:  node.Reputation.UptimeReputationBeta,
			ReputationScore: uptimeScore,
		},
		AuditCheck: &pb.ReputationStats{
			TotalCount:      node.Reputation.AuditCount,
			SuccessCount:    node.Reputation.AuditSuccessCount,
			ReputationAlpha: node.Reputation.AuditReputationAlpha,
			ReputationBeta:  node.Reputation.AuditReputationBeta,
			ReputationScore: auditScore,
		},
		Disqualified: node.Disqualified,
	}, nil
}

// DailyStorageUsage returns slice of daily storage usage for given period of time sorted in ASC order by date
func (e *Endpoint) DailyStorageUsage(ctx context.Context, req *pb.DailyStorageUsageRequest) (_ *pb.DailyStorageUsageResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			return nil, rpcstatus.Error(rpcstatus.PermissionDenied, err.Error())
		}
		e.log.Error("overlay.Get failed", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	nodeSpaceUsages, err := e.accounting.QueryStorageNodeUsage(ctx, node.Id, req.GetFrom(), req.GetTo())
	if err != nil {
		e.log.Error("accounting.QueryStorageNodeUsage failed", zap.Error(err))
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &pb.DailyStorageUsageResponse{
		NodeId:            node.Id,
		DailyStorageUsage: toProtoDailyStorageUsage(nodeSpaceUsages),
	}, nil
}

// toProtoDailyStorageUsage converts StorageNodeUsage to PB DailyStorageUsageResponse_StorageUsage
func toProtoDailyStorageUsage(usages []accounting.StorageNodeUsage) []*pb.DailyStorageUsageResponse_StorageUsage {
	var pbUsages []*pb.DailyStorageUsageResponse_StorageUsage

	for _, usage := range usages {
		pbUsages = append(pbUsages, &pb.DailyStorageUsageResponse_StorageUsage{
			AtRestTotal: usage.StorageUsed,
			Timestamp:   usage.Timestamp,
		})
	}

	return pbUsages
}

// calculateReputationScore is helper method to calculate reputation score value
func calculateReputationScore(alpha, beta float64) float64 {
	return alpha / (alpha + beta)
}
