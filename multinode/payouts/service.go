// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodepb"
)

var (
	mon = monkit.Package()
	// Error is an error class for payouts service error.
	Error = errs.Class("payouts")
)

// Service exposes all payouts related logic.
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer
	nodes  nodes.DB
}

// NewService creates new instance of Service.
func NewService(log *zap.Logger, dialer rpc.Dialer, nodes nodes.DB) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
		nodes:  nodes,
	}
}

// GetAllNodesAllTimeEarned retrieves all nodes earned amount for all time.
func (service *Service) GetAllNodesAllTimeEarned(ctx context.Context) (earned int64, err error) {
	defer mon.Task()(&ctx)(&err)

	storageNodes, err := service.nodes.List(ctx)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	for _, node := range storageNodes {
		amount, err := service.getAmount(ctx, node)
		if err != nil {
			service.log.Error("failed to getAmount", zap.Error(err))
			continue
		}

		earned += amount
	}

	return earned, nil
}

// GetAllNodesEarnedOnSatellite retrieves all nodes earned amount for all time per satellite.
func (service *Service) GetAllNodesEarnedOnSatellite(ctx context.Context) (earned []SatelliteSummary, err error) {
	defer mon.Task()(&ctx)(&err)

	storageNodes, err := service.nodes.List(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var listSatellites storj.NodeIDList
	var listNodesEarnedPerSatellite []multinodepb.EarnedPerSatelliteResponse

	for _, node := range storageNodes {
		earnedPerSatellite, err := service.getEarnedOnSatellite(ctx, node)
		if err != nil {
			service.log.Error("failed to getEarnedFromSatellite", zap.Error(err))
			continue
		}

		listNodesEarnedPerSatellite = append(listNodesEarnedPerSatellite, earnedPerSatellite)
		for i := 0; i < len(earnedPerSatellite.EarnedSatellite); i++ {
			listSatellites = append(listSatellites, earnedPerSatellite.EarnedSatellite[i].SatelliteId)
		}
	}

	if listSatellites == nil {
		return []SatelliteSummary{}, nil
	}

	uniqueSatelliteIDs := listSatellites.Unique()
	for t := 0; t < len(uniqueSatelliteIDs); t++ {
		earned = append(earned, SatelliteSummary{
			SatelliteID: uniqueSatelliteIDs[t],
		})
	}

	for i := 0; i < len(listNodesEarnedPerSatellite); i++ {
		singleNodeEarnedPerSatellite := listNodesEarnedPerSatellite[i].EarnedSatellite
		for j := 0; j < len(singleNodeEarnedPerSatellite); j++ {
			for k := 0; k < len(earned); k++ {
				if singleNodeEarnedPerSatellite[j].SatelliteId == earned[k].SatelliteID {
					earned[k].Earned += singleNodeEarnedPerSatellite[j].Total
				}
			}
		}
	}

	return earned, nil
}

func (service *Service) getAmount(ctx context.Context, node nodes.Node) (_ int64, err error) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return 0, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	amount, err := payoutClient.Earned(ctx, &multinodepb.EarnedRequest{Header: header})
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return amount.Total, nil
}

func (service *Service) getEarnedOnSatellite(ctx context.Context, node nodes.Node) (_ multinodepb.EarnedPerSatelliteResponse, err error) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return multinodepb.EarnedPerSatelliteResponse{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.EarnedPerSatellite(ctx, &multinodepb.EarnedPerSatelliteRequest{Header: header})
	if err != nil {
		return multinodepb.EarnedPerSatelliteResponse{}, Error.Wrap(err)
	}

	return *response, nil
}
