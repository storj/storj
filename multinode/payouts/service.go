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
	"storj.io/drpc"
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

// NodesSummary returns all satellites all time stats.
func (service *Service) NodesSummary(ctx context.Context) (_ Summary, err error) {
	defer mon.Task()(&ctx)(&err)

	var summary Summary

	list, err := service.nodes.List(ctx)
	if err != nil {
		return Summary{}, Error.Wrap(err)
	}

	for _, node := range list {
		info, err := service.getAllSatellitesAllTime(ctx, node)
		if err != nil {
			return Summary{}, Error.Wrap(err)
		}

		summary.Add(info.Held, info.Paid, node.ID, node.Name)
	}

	return summary, nil
}

// NodesPeriodSummary returns all satellites stats for specific period.
func (service *Service) NodesPeriodSummary(ctx context.Context, period string) (_ Summary, err error) {
	defer mon.Task()(&ctx)(&err)

	var summary Summary

	list, err := service.nodes.List(ctx)
	if err != nil {
		return Summary{}, Error.Wrap(err)
	}

	for _, node := range list {
		info, err := service.getAllSatellitesPeriod(ctx, node, period)
		if err != nil {
			return Summary{}, Error.Wrap(err)
		}

		summary.Add(info.Held, info.Paid, node.ID, node.Name)
	}

	return summary, nil
}

// NodesSatelliteSummary returns specific satellite all time stats.
func (service *Service) NodesSatelliteSummary(ctx context.Context, satelliteID storj.NodeID) (_ Summary, err error) {
	defer mon.Task()(&ctx)(&err)
	var summary Summary

	list, err := service.nodes.List(ctx)
	if err != nil {
		return Summary{}, Error.Wrap(err)
	}

	for _, node := range list {
		info, err := service.nodeSatelliteSummary(ctx, node, satelliteID)
		if err != nil {
			return Summary{}, Error.Wrap(err)
		}

		summary.Add(info.Held, info.Paid, node.ID, node.Name)
	}

	return summary, nil
}

// NodesSatellitePeriodSummary returns specific satellite stats for specific period.
func (service *Service) NodesSatellitePeriodSummary(ctx context.Context, satelliteID storj.NodeID, period string) (_ Summary, err error) {
	defer mon.Task()(&ctx)(&err)
	var summary Summary

	list, err := service.nodes.List(ctx)
	if err != nil {
		return Summary{}, Error.Wrap(err)
	}

	for _, node := range list {
		info, err := service.nodeSatellitePeriodSummary(ctx, node, satelliteID, period)
		if err != nil {
			return Summary{}, Error.Wrap(err)
		}

		summary.Add(info.Held, info.Paid, node.ID, node.Name)
	}

	return summary, nil
}

// nodeSatelliteSummary returns payout info for single satellite, for specific node.
func (service *Service) nodeSatelliteSummary(ctx context.Context, node nodes.Node, satelliteID storj.NodeID) (info *multinodepb.PayoutInfo, err error) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.SatelliteSummary(ctx, &multinodepb.SatelliteSummaryRequest{Header: header, SatelliteId: satelliteID})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	return response.PayoutInfo, nil
}

// nodeSatellitePeriodSummary returns satellite payout info for specific node for specific period.
func (service *Service) nodeSatellitePeriodSummary(ctx context.Context, node nodes.Node, satelliteID storj.NodeID, period string) (info *multinodepb.PayoutInfo, err error) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.SatellitePeriodSummary(ctx, &multinodepb.SatellitePeriodSummaryRequest{Header: header, SatelliteId: satelliteID, Period: period})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	return response.PayoutInfo, nil
}

func (service *Service) getAllSatellitesPeriod(ctx context.Context, node nodes.Node, period string) (info *multinodepb.PayoutInfo, err error) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.AllSatellitesPeriodSummary(ctx, &multinodepb.AllSatellitesPeriodSummaryRequest{Header: header, Period: period})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	return response.PayoutInfo, nil
}

func (service *Service) getAllSatellitesAllTime(ctx context.Context, node nodes.Node) (info *multinodepb.PayoutInfo, err error) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.AllSatellitesSummary(ctx, &multinodepb.AllSatellitesSummaryRequest{Header: header})
	if err != nil {
		return &multinodepb.PayoutInfo{}, Error.Wrap(err)
	}

	return response.PayoutInfo, nil
}

// NodeExpectations returns node's estimated and undistributed earnings.
func (service *Service) NodeExpectations(ctx context.Context, nodeID storj.NodeID) (_ Expectations, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return Expectations{}, Error.Wrap(err)
	}

	expectation, err := service.nodeExpectations(ctx, node)
	if err != nil {
		return Expectations{}, Error.Wrap(err)
	}

	return expectation, nil
}

// Expectations returns all nodes estimated and undistributed earnings.
func (service *Service) Expectations(ctx context.Context) (_ Expectations, err error) {
	defer mon.Task()(&ctx)(&err)

	var expectations Expectations

	list, err := service.nodes.List(ctx)
	if err != nil {
		return Expectations{}, Error.Wrap(err)
	}

	for _, node := range list {
		expectation, err := service.nodeExpectations(ctx, node)
		if err != nil {
			return Expectations{}, Error.Wrap(err)
		}

		expectations.Undistributed += expectation.Undistributed
		expectations.CurrentMonthEstimation += expectation.CurrentMonthEstimation
	}

	return expectations, nil
}

// HeldAmountSummary retrieves held amount history summary for a particular node.
func (service *Service) HeldAmountSummary(ctx context.Context, nodeID storj.NodeID) (_ []HeldAmountSummary, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	nodeClient := multinodepb.NewDRPCNodeClient(conn)

	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}
	trusted, err := nodeClient.TrustedSatellites(ctx, &multinodepb.TrustedSatellitesRequest{
		Header: header,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	history, err := service.heldAmountHistory(ctx, node, conn)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	trustedSatellites := trusted.GetTrustedSatellites()

	var summary []HeldAmountSummary
	for _, satelliteHistory := range history {
		satelliteSummary := HeldAmountSummary{
			SatelliteID: satelliteHistory.SatelliteID,
		}

		for _, trustedSatellite := range trustedSatellites {
			if satelliteSummary.SatelliteID.Compare(trustedSatellite.NodeId) == 0 {
				satelliteSummary.SatelliteURL = storj.NodeURL{
					ID:      trustedSatellite.NodeId,
					Address: trustedSatellite.GetAddress(),
				}
			}
		}

		if len(satelliteHistory.HeldAmounts) == 0 {
			summary = append(summary, satelliteSummary)
			continue
		}

		satelliteSummary.PeriodCount = len(satelliteHistory.HeldAmounts)

		for i, heldAmount := range satelliteHistory.HeldAmounts {
			switch i {
			case 1, 2, 3:
				satelliteSummary.FirstQuarter += heldAmount.Amount
			case 4, 5, 6:
				satelliteSummary.SecondQuarter += heldAmount.Amount
			case 7, 8, 9:
				satelliteSummary.ThirdQuarter += heldAmount.Amount
			}
		}

		summary = append(summary, satelliteSummary)
	}

	return summary, nil
}

// heldAmountHistory retrieves held amount history for a particular node.
func (service *Service) heldAmountHistory(ctx context.Context, node nodes.Node, conn drpc.Conn) (_ []HeldAmountHistory, err error) {
	defer mon.Task()(&ctx)(&err)
	payoutClient := multinodepb.NewDRPCPayoutClient(conn)

	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}
	resp, err := payoutClient.HeldAmountHistory(ctx, &multinodepb.HeldAmountHistoryRequest{
		Header: header,
	})
	if err != nil {
		return nil, err
	}

	var history []HeldAmountHistory
	for _, pbHistory := range resp.GetHistory() {
		var heldAmounts []HeldAmount

		for _, pbHeldAmount := range pbHistory.GetHeldAmounts() {
			heldAmounts = append(heldAmounts, HeldAmount{
				Period: pbHeldAmount.GetPeriod(),
				Amount: pbHeldAmount.GetAmount(),
			})
		}

		history = append(history, HeldAmountHistory{
			SatelliteID: pbHistory.SatelliteId,
			HeldAmounts: heldAmounts,
		})
	}

	return history, nil
}

// nodeExpectations retrieves data from a single node.
func (service *Service) nodeExpectations(ctx context.Context, node nodes.Node) (_ Expectations, err error) {
	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Expectations{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	estimated, err := payoutClient.EstimatedPayoutTotal(ctx, &multinodepb.EstimatedPayoutTotalRequest{Header: header})
	if err != nil {
		return Expectations{}, Error.Wrap(err)
	}

	undistributed, err := payoutClient.Undistributed(ctx, &multinodepb.UndistributedRequest{Header: header})
	if err != nil {
		return Expectations{}, Error.Wrap(err)
	}

	return Expectations{Undistributed: undistributed.Total, CurrentMonthEstimation: estimated.EstimatedEarnings}, nil
}

// getAmount returns earned from node.
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

// getEarnedOnSatellite returns earned split by satellites.
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

// SatellitePaystubPeriod returns specific satellite paystub for specific period.
func (service *Service) SatellitePaystubPeriod(ctx context.Context, period string, nodeID, satelliteID storj.NodeID) (_ Paystub, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.SatellitePeriodPaystub(ctx, &multinodepb.SatellitePeriodPaystubRequest{
		Header:      header,
		SatelliteId: satelliteID,
		Period:      period,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	return Paystub{
		UsageAtRest:    response.Paystub.UsageAtRest,
		UsageGet:       response.Paystub.UsageGet,
		UsageGetRepair: response.Paystub.UsageGetRepair,
		UsageGetAudit:  response.Paystub.UsageGetAudit,
		CompAtRest:     response.Paystub.CompAtRest,
		CompGet:        response.Paystub.CompGet,
		CompGetRepair:  response.Paystub.CompGetRepair,
		CompGetAudit:   response.Paystub.CompGetAudit,
		Held:           response.Paystub.Held,
		Paid:           response.Paystub.Paid,
		Distributed:    response.Paystub.Distributed,
	}, nil
}

// PaystubPeriod returns all satellites paystub for specific period.
func (service *Service) PaystubPeriod(ctx context.Context, period string, nodeID storj.NodeID) (_ Paystub, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.PeriodPaystub(ctx, &multinodepb.PeriodPaystubRequest{
		Header: header,
		Period: period,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	return Paystub{
		UsageAtRest:    response.Paystub.UsageAtRest,
		UsageGet:       response.Paystub.UsageGet,
		UsageGetRepair: response.Paystub.UsageGetRepair,
		UsageGetAudit:  response.Paystub.UsageGetAudit,
		CompAtRest:     response.Paystub.CompAtRest,
		CompGet:        response.Paystub.CompGet,
		CompGetRepair:  response.Paystub.CompGetRepair,
		CompGetAudit:   response.Paystub.CompGetAudit,
		Held:           response.Paystub.Held,
		Paid:           response.Paystub.Paid,
		Distributed:    response.Paystub.Distributed,
	}, nil
}

// SatellitePaystub returns specific satellite summed paystubs.
func (service *Service) SatellitePaystub(ctx context.Context, nodeID, satelliteID storj.NodeID) (_ Paystub, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.SatellitePaystub(ctx, &multinodepb.SatellitePaystubRequest{
		Header:      header,
		SatelliteId: satelliteID,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	return Paystub{
		UsageAtRest:    response.Paystub.UsageAtRest,
		UsageGet:       response.Paystub.UsageGet,
		UsageGetRepair: response.Paystub.UsageGetRepair,
		UsageGetAudit:  response.Paystub.UsageGetAudit,
		CompAtRest:     response.Paystub.CompAtRest,
		CompGet:        response.Paystub.CompGet,
		CompGetRepair:  response.Paystub.CompGetRepair,
		CompGetAudit:   response.Paystub.CompGetAudit,
		Held:           response.Paystub.Held,
		Paid:           response.Paystub.Paid,
		Distributed:    response.Paystub.Distributed,
	}, nil
}

// Paystub returns summed all paystubs.
func (service *Service) Paystub(ctx context.Context, nodeID storj.NodeID) (_ Paystub, err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.nodes.Get(ctx, nodeID)
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, storj.NodeURL{
		ID:      node.ID,
		Address: node.PublicAddress,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	payoutClient := multinodepb.NewDRPCPayoutClient(conn)
	header := &multinodepb.RequestHeader{
		ApiKey: node.APISecret,
	}

	response, err := payoutClient.Paystub(ctx, &multinodepb.PaystubRequest{
		Header: header,
	})
	if err != nil {
		return Paystub{}, Error.Wrap(err)
	}

	return Paystub{
		UsageAtRest:    response.Paystub.UsageAtRest,
		UsageGet:       response.Paystub.UsageGet,
		UsageGetRepair: response.Paystub.UsageGetRepair,
		UsageGetAudit:  response.Paystub.UsageGetAudit,
		CompAtRest:     response.Paystub.CompAtRest,
		CompGet:        response.Paystub.CompGet,
		CompGetRepair:  response.Paystub.CompGetRepair,
		CompGetAudit:   response.Paystub.CompGetAudit,
		Held:           response.Paystub.Held,
		Paid:           response.Paystub.Paid,
		Distributed:    response.Paystub.Distributed,
	}, nil
}
