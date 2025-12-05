// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodepb"
)

var (
	mon = monkit.Package()
	// Error is an error class for reputation service error.
	Error = errs.Class("reputation")
	// ErrorNoStats is an error class for reputation is not found error.
	ErrorNoStats = errs.Class("reputation stats not found")
)

// Service exposes all reputation related logic.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer
	nodes  nodes.DB
}

// NewService creates new instance of reputation Service.
func NewService(log *zap.Logger, dialer rpc.Dialer, nodes nodes.DB) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
		nodes:  nodes,
	}
}

// Stats retrieves node reputation stats list for satellite.
func (service *Service) Stats(ctx context.Context, satelliteID storj.NodeID) (_ []Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeList, err := service.nodes.List(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var statsList []Stats
	for _, node := range nodeList {
		stats, err := service.dialStats(ctx, node, satelliteID)
		if err != nil {
			if ErrorNoStats.Has(err) {
				continue
			}

			if nodes.ErrNodeNotReachable.Has(err) {
				continue
			}

			return nil, Error.Wrap(err)
		}

		statsList = append(statsList, stats)
	}

	return statsList, nil
}

// dialStats dials node and retrieves reputation stats for particular satellite.
func (service *Service) dialStats(ctx context.Context, node nodes.Node, satelliteID storj.NodeID) (_ Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Stats{}, nodes.ErrNodeNotReachable.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	nodeClient := multinodepb.NewDRPCNodeClient(conn)

	req := &multinodepb.ReputationRequest{
		Header: &multinodepb.RequestHeader{
			ApiKey: node.APISecret[:],
		},
		SatelliteId: satelliteID,
	}
	resp, err := nodeClient.Reputation(ctx, req)
	if err != nil {
		if rpcstatus.Code(err) == rpcstatus.NotFound {
			return Stats{}, ErrorNoStats.New("no stats for %s", satelliteID.String())
		}
		return Stats{}, Error.Wrap(err)
	}

	var auditWindows []AuditWindow
	for _, window := range resp.Audit.History {
		auditWindows = append(auditWindows, AuditWindow{
			WindowStart: window.WindowStart,
			TotalCount:  window.TotalCount,
			OnlineCount: window.OnlineCount,
		})
	}

	return Stats{
		NodeID:   node.ID,
		NodeName: node.Name,
		Audit: Audit{
			TotalCount:      resp.Audit.TotalCount,
			SuccessCount:    resp.Audit.SuccessCount,
			Alpha:           resp.Audit.Alpha,
			Beta:            resp.Audit.Beta,
			UnknownAlpha:    resp.Audit.UnknownAlpha,
			UnknownBeta:     resp.Audit.UnknownBeta,
			Score:           resp.Audit.Score,
			SuspensionScore: resp.Audit.SuspensionScore,
			History:         auditWindows,
		},
		OnlineScore:          resp.Online.Score,
		DisqualifiedAt:       resp.DisqualifiedAt,
		SuspendedAt:          resp.SuspendedAt,
		OfflineSuspendedAt:   resp.OfflineSuspendedAt,
		OfflineUnderReviewAt: resp.OfflineUnderReviewAt,
		VettedAt:             resp.VettedAt,
		UpdatedAt:            resp.UpdatedAt,
		JoinedAt:             resp.JoinedAt,
	}, nil
}
