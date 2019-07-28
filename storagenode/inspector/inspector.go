// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
)

var (
	mon = monkit.Package()

	// Error is the default error class for piecestore monitor errors
	Error = errs.Class("piecestore inspector")
)

// Endpoint does inspectory things
type Endpoint struct {
	log       *zap.Logger
	pieceInfo pieces.DB
	kademlia  *kademlia.Kademlia
	usageDB   bandwidth.DB

	startTime        time.Time
	pieceStoreConfig piecestore.OldConfig
	dashboardAddress net.Addr
}

// NewEndpoint creates piecestore inspector instance
func NewEndpoint(
	log *zap.Logger,
	pieceInfo pieces.DB,
	kademlia *kademlia.Kademlia,
	usageDB bandwidth.DB,
	pieceStoreConfig piecestore.OldConfig,
	dashbaordAddress net.Addr) *Endpoint {

	return &Endpoint{
		log:              log,
		pieceInfo:        pieceInfo,
		kademlia:         kademlia,
		usageDB:          usageDB,
		pieceStoreConfig: pieceStoreConfig,
		dashboardAddress: dashbaordAddress,
		startTime:        time.Now(),
	}
}

func (inspector *Endpoint) retrieveStats(ctx context.Context) (_ *pb.StatSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	// Space Usage
	totalUsedSpace, err := inspector.pieceInfo.SpaceUsed(ctx)
	if err != nil {
		return nil, err
	}
	usage, err := bandwidth.TotalMonthlySummary(ctx, inspector.usageDB)
	if err != nil {
		return nil, err
	}
	ingress := usage.Put + usage.PutRepair
	egress := usage.Get + usage.GetAudit + usage.GetRepair

	totalUsedBandwidth := usage.Total()

	return &pb.StatSummaryResponse{
		UsedSpace:          totalUsedSpace,
		AvailableSpace:     inspector.pieceStoreConfig.AllocatedDiskSpace.Int64() - totalUsedSpace,
		UsedIngress:        ingress,
		UsedEgress:         egress,
		UsedBandwidth:      totalUsedBandwidth,
		AvailableBandwidth: inspector.pieceStoreConfig.AllocatedBandwidth.Int64() - totalUsedBandwidth,
	}, nil
}

// Stats returns current statistics about the storage node
func (inspector *Endpoint) Stats(ctx context.Context, in *pb.StatsRequest) (out *pb.StatSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	inspector.log.Debug("Getting Stats...")

	statsSummary, err := inspector.retrieveStats(ctx)
	if err != nil {
		return nil, err
	}

	inspector.log.Info("Successfully retrieved Stats...")

	return statsSummary, nil
}

func (inspector *Endpoint) getDashboardData(ctx context.Context) (_ *pb.DashboardResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	statsSummary, err := inspector.retrieveStats(ctx)
	if err != nil {
		return &pb.DashboardResponse{}, Error.Wrap(err)
	}

	// TODO: querying all nodes is slow, find a more performant way to do this.
	nodes, err := inspector.kademlia.FindNear(ctx, storj.NodeID{}, 10000000)
	if err != nil {
		return &pb.DashboardResponse{}, Error.Wrap(err)
	}

	bootstrapNodes := inspector.kademlia.GetBootstrapNodes()
	bsNodes := make([]string, len(bootstrapNodes))
	for i, node := range bootstrapNodes {
		bsNodes[i] = node.Address.Address
	}

	return &pb.DashboardResponse{
		NodeId:           inspector.kademlia.Local().Id,
		NodeConnections:  int64(len(nodes)),
		BootstrapAddress: strings.Join(bsNodes, ", "),
		InternalAddress:  "",
		ExternalAddress:  inspector.kademlia.Local().Address.Address,
		LastPinged:       inspector.kademlia.LastPinged(),
		LastQueried:      inspector.kademlia.LastQueried(),
		DashboardAddress: inspector.dashboardAddress.String(),
		Uptime:           ptypes.DurationProto(time.Since(inspector.startTime)),
		Stats:            statsSummary,
	}, nil
}

// Dashboard returns dashboard information
func (inspector *Endpoint) Dashboard(ctx context.Context, in *pb.DashboardRequest) (out *pb.DashboardResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	data, err := inspector.getDashboardData(ctx)
	if err != nil {
		inspector.log.Warn("unable to get dashboard information")
		return nil, err
	}
	return data, nil
}
