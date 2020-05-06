// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"
	"net"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
)

var (
	mon = monkit.Package()

	// Error is the default error class for piecestore monitor errors
	Error = errs.Class("piecestore inspector")
)

// Endpoint does inspectory things
//
// architecture: Endpoint
type Endpoint struct {
	log        *zap.Logger
	pieceStore *pieces.Store
	contact    *contact.Service
	pingStats  *contact.PingStats
	usageDB    bandwidth.DB

	startTime        time.Time
	pieceStoreConfig piecestore.OldConfig
	dashboardAddress net.Addr
	externalAddress  string
}

// NewEndpoint creates piecestore inspector instance
func NewEndpoint(
	log *zap.Logger,
	pieceStore *pieces.Store,
	contact *contact.Service,
	pingStats *contact.PingStats,
	usageDB bandwidth.DB,
	pieceStoreConfig piecestore.OldConfig,
	dashboardAddress net.Addr,
	externalAddress string) *Endpoint {

	return &Endpoint{
		log:              log,
		pieceStore:       pieceStore,
		contact:          contact,
		pingStats:        pingStats,
		usageDB:          usageDB,
		pieceStoreConfig: pieceStoreConfig,
		dashboardAddress: dashboardAddress,
		startTime:        time.Now(),
		externalAddress:  externalAddress,
	}
}

// Stats returns current statistics about the storage node
func (inspector *Endpoint) Stats(ctx context.Context, in *pb.StatsRequest) (out *pb.StatSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return inspector.retrieveStats(ctx)
}

func (inspector *Endpoint) retrieveStats(ctx context.Context) (_ *pb.StatSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	// Space Usage
	piecesContentSize, err := inspector.pieceStore.SpaceUsedForPiecesAndTrash(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	usage, err := bandwidth.TotalMonthlySummary(ctx, inspector.usageDB)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	ingress := usage.Put + usage.PutRepair
	egress := usage.Get + usage.GetAudit + usage.GetRepair

	totalUsedBandwidth := usage.Total()

	return &pb.StatSummaryResponse{
		UsedSpace:      piecesContentSize,
		AvailableSpace: inspector.pieceStoreConfig.AllocatedDiskSpace.Int64() - piecesContentSize,
		UsedIngress:    ingress,
		UsedEgress:     egress,
		UsedBandwidth:  totalUsedBandwidth,
	}, nil
}

// Dashboard returns dashboard information
func (inspector *Endpoint) Dashboard(ctx context.Context, in *pb.DashboardRequest) (out *pb.DashboardResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return inspector.getDashboardData(ctx)
}

func (inspector *Endpoint) getDashboardData(ctx context.Context) (_ *pb.DashboardResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	statsSummary, err := inspector.retrieveStats(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	lastPingedAt := inspector.pingStats.WhenLastPinged()

	return &pb.DashboardResponse{
		NodeId:           inspector.contact.Local().Id,
		InternalAddress:  "",
		ExternalAddress:  inspector.contact.Local().Address.Address,
		LastPinged:       lastPingedAt,
		DashboardAddress: inspector.dashboardAddress.String(),
		Uptime:           time.Since(inspector.startTime).String(),
		Stats:            statsSummary,
	}, nil
}
