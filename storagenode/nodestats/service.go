// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/storageusage"
	"storj.io/storj/storagenode/trust"
)

var (
	// NodeStatsServiceErr defines node stats service error.
	NodeStatsServiceErr = errs.Class("nodestats")

	mon = monkit.Package()
)

// Client encapsulates NodeStatsClient with underlying connection.
//
// architecture: Client
type Client struct {
	conn *rpc.Conn
	pb.DRPCNodeStatsClient
}

// Close closes underlying client connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Service retrieves info from satellites using an rpc client.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	dialer rpc.Dialer
	trust  *trust.Pool
}

// NewService creates new instance of service.
func NewService(log *zap.Logger, dialer rpc.Dialer, trust *trust.Pool) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
		trust:  trust,
	}
}

// GetDailyStorageUsage returns daily storage usage over a period of time for a particular satellite.
func (s *Service) GetDailyStorageUsage(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.dial(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	resp, err := client.DailyStorageUsage(ctx, &pb.DailyStorageUsageRequest{From: from, To: to})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return fromSpaceUsageResponse(resp, satelliteID), nil
}

// GetPricingModel returns pricing model of specific satellite.
func (s *Service) GetPricingModel(ctx context.Context, satelliteID storj.NodeID) (_ *pricing.Pricing, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := s.dial(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	pricingModel, err := client.PricingModel(ctx, &pb.PricingModelRequest{})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return &pricing.Pricing{
		SatelliteID:     satelliteID,
		EgressBandwidth: pricingModel.EgressBandwidthPrice,
		RepairBandwidth: pricingModel.RepairBandwidthPrice,
		AuditBandwidth:  pricingModel.AuditBandwidthPrice,
		DiskSpace:       pricingModel.DiskSpacePrice,
	}, nil
}

// dial dials the NodeStats client for the satellite by id.
func (s *Service) dial(ctx context.Context, satelliteID storj.NodeID) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeurl, err := s.trust.GetNodeURL(ctx, satelliteID)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %w", satelliteID, err)
	}

	conn, err := s.dialer.DialNodeURL(ctx, nodeurl)
	if err != nil {
		return nil, errs.New("unable to connect to the satellite %s: %w", satelliteID, err)
	}

	return &Client{
		conn:                conn,
		DRPCNodeStatsClient: pb.NewDRPCNodeStatsClient(conn),
	}, nil
}

// fromSpaceUsageResponse get DiskSpaceUsage slice from pb.SpaceUsageResponse.
func fromSpaceUsageResponse(resp *pb.DailyStorageUsageResponse, satelliteID storj.NodeID) []storageusage.Stamp {
	var stamps []storageusage.Stamp

	for _, pbUsage := range resp.GetDailyStorageUsage() {
		stamps = append(stamps, storageusage.Stamp{
			SatelliteID:     satelliteID,
			AtRestTotal:     pbUsage.AtRestTotal,
			IntervalStart:   pbUsage.Timestamp,
			IntervalEndTime: pbUsage.IntervalEndTime,
		})
	}

	return stamps
}
