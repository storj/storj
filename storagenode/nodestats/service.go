// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	// NodeStatsServiceErr defines node stats service error
	NodeStatsServiceErr = errs.Class("node stats service error")

	mon = monkit.Package()
)

// UptimeCheck encapsulates storagenode uptime metrics
type UptimeCheck struct {
	TotalCount   int64
	SuccessCount int64

	ReputationAlpha float64
	ReputationBeta  float64
	ReputationScore float64
}

// Service retrieves info from satellites using GRPC client
type Service struct {
	log *zap.Logger

	transport transport.Client
	kademlia  *kademlia.Kademlia
}

// NewService creates new instance of service
func NewService(log *zap.Logger, transport transport.Client, kademlia *kademlia.Kademlia) *Service {
	return &Service{
		log:       log,
		transport: transport,
		kademlia:  kademlia,
	}
}

// GetUptimeCheckForSatellite retrieves UptimeChecks from particular satellite
func (s *Service) GetUptimeCheckForSatellite(ctx context.Context, satelliteID storj.NodeID) (_ *UptimeCheck, err error) {
	defer mon.Task()(&ctx)(&err)

	satellite, err := s.kademlia.FindNode(ctx, satelliteID)
	if err != nil {
		return nil, NodeStatsServiceErr.New("unable to find satellite %s: %v", satelliteID, err)
	}

	conn, err := s.transport.DialNode(ctx, &satellite)
	if err != nil {
		return nil, NodeStatsServiceErr.New("unable to connect to the satellite %s: %v", satelliteID, err)
	}

	client := pb.NewNodeStatsClient(conn)

	resp, err := client.UptimeCheck(ctx, &pb.UptimeCheckRequest{})
	if err != nil {
		return nil, NodeStatsServiceErr.Wrap(err)
	}

	return &UptimeCheck{
		TotalCount:      resp.GetTotalCount(),
		SuccessCount:    resp.GetSuccessCount(),
		ReputationAlpha: resp.GetReputationAlpha(),
		ReputationBeta:  resp.GetReputationBeta(),
		ReputationScore: resp.GetReputationScore(),
	}, nil
}
