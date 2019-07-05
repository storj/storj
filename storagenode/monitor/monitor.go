// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/pieces"
)

var (
	mon = monkit.Package()

	// Error is the default error class for piecestore monitor errors
	Error = errs.Class("piecestore monitor")
)

// Config defines parameters for storage node disk and bandwidth usage monitoring.
type Config struct {
	Interval         time.Duration `help:"how frequently Kademlia bucket should be refreshed with node stats" default:"1h0m0s"`
	MinimumDiskSpace memory.Size   `help:"how much disk space a node at minimum has to advertise" default:"500GB"`
	MinimumBandwidth memory.Size   `help:"how much bandwidth a node at minimum has to advertise" default:"500GB"`
}

// Service which monitors disk usage and updates kademlia network as necessary.
type Service struct {
	log                *zap.Logger
	routingTable       *kademlia.RoutingTable
	store              *pieces.Store
	pieceInfo          pieces.DB
	usageDB            bandwidth.DB
	allocatedDiskSpace int64
	allocatedBandwidth int64
	Loop               sync2.Cycle
	Config             Config
	status             Status
}

// Status of bandwidth and storage for a storagenode
type Status struct {
	usedBandwidth int64
	bwMutex       sync.RWMutex

	usedSpace int64
	sMutex    sync.RWMutex
}

// TODO: should it be responsible for monitoring actual bandwidth as well?

// NewService creates a new storage node monitoring service.
func NewService(log *zap.Logger, routingTable *kademlia.RoutingTable, store *pieces.Store, pieceInfo pieces.DB, usageDB bandwidth.DB, allocatedDiskSpace, allocatedBandwidth int64, interval time.Duration, config Config) *Service {
	return &Service{
		log:                log,
		routingTable:       routingTable,
		store:              store,
		pieceInfo:          pieceInfo,
		usageDB:            usageDB,
		allocatedDiskSpace: allocatedDiskSpace,
		allocatedBandwidth: allocatedBandwidth,
		Loop:               *sync2.NewCycle(interval),
		Config:             config,
	}
}

// Run runs monitor service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// get the disk space details
	// The returned path ends in a slash only if it represents a root directory, such as "/" on Unix or `C:\` on Windows.
	storageStatus, err := service.store.StorageStatus(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	freeDiskSpace := storageStatus.DiskFree

	totalUsed, err := service.usedSpace(ctx)
	if err != nil {
		return err
	}
	service.SetUsedSpace(totalUsed)

	usedBandwidth, err := service.usedBandwidth(ctx)
	if err != nil {
		return err
	}
	service.SetUsedBandwidth(usedBandwidth)

	if usedBandwidth > service.allocatedBandwidth {
		service.log.Warn("Exceed the allowed Bandwidth setting")
	} else {
		service.log.Info("Remaining Bandwidth", zap.Int64("bytes", service.allocatedBandwidth-usedBandwidth))
	}

	// check your hard drive is big enough
	// first time setup as a piece node server
	if totalUsed == 0 && freeDiskSpace < service.allocatedDiskSpace {
		service.allocatedDiskSpace = freeDiskSpace
		service.log.Warn("Disk space is less than requested. Allocating space", zap.Int64("bytes", service.allocatedDiskSpace))
	}

	// on restarting the Piece node server, assuming already been working as a node
	// used above the alloacated space, user changed the allocation space setting
	// before restarting
	if totalUsed >= service.allocatedDiskSpace {
		service.log.Warn("Used more space than allocated. Allocating space", zap.Int64("bytes", service.allocatedDiskSpace))
	}

	// the available disk space is less than remaining allocated space,
	// due to change of setting before restarting
	if freeDiskSpace < service.allocatedDiskSpace-totalUsed {
		service.allocatedDiskSpace = freeDiskSpace + totalUsed
		service.log.Warn("Disk space is less than requested. Allocating space", zap.Int64("bytes", service.allocatedDiskSpace))
	}

	// Ensure the disk is at least 500GB in size, which is our current minimum required to be an operator
	if service.allocatedDiskSpace < service.Config.MinimumDiskSpace.Int64() {
		service.log.Error("Total disk space less than required minimum", zap.Int64("bytes", service.Config.MinimumDiskSpace.Int64()))
		return Error.New("disk space requirement not met")
	}

	// Ensure the bandwidth is at least 500GB
	if service.allocatedBandwidth < service.Config.MinimumBandwidth.Int64() {
		service.log.Error("Total Bandwidth available less than required minimum", zap.Int64("bytes", service.Config.MinimumBandwidth.Int64()))
		return Error.New("bandwidth requirement not met")
	}

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.updateNodeInformation(ctx)
		if err != nil {
			service.log.Error("error during updating node information: ", zap.Error(err))
		}
		return err
	})
}

// GetUsedBandwidth returns the used bandwidth for a storagenode
func (service *Service) GetUsedBandwidth() int64 {
	service.status.bwMutex.RLock()
	defer service.status.bwMutex.RUnlock()

	return service.status.usedBandwidth
}

// SetUsedBandwidth sets the used bandwidth for a storagenode
func (service *Service) SetUsedBandwidth(used int64) {
	service.status.bwMutex.Lock()
	defer service.status.bwMutex.Unlock()
	service.log.Sugar().Debugf("setting used bandwidth to %d\n", used)
	service.status.usedBandwidth = used
}

// UpdateUsedBandwidth add/subtract used bandwidth for a storagenode
func (service *Service) UpdateUsedBandwidth(used int64) {
	service.status.bwMutex.Lock()
	defer service.status.bwMutex.Unlock()
	service.log.Sugar().Debugf("update used bandwidth with %d\n", used)
	service.status.usedBandwidth += used
}

// GetUsedSpace returns the used space for a storagenode
func (service *Service) GetUsedSpace() int64 {
	service.status.sMutex.RLock()
	defer service.status.sMutex.RUnlock()
	return service.status.usedSpace
}

// SetUsedSpace sets the used space for a storagenode
func (service *Service) SetUsedSpace(used int64) {
	service.status.sMutex.Lock()
	defer service.status.sMutex.Unlock()
	service.log.Sugar().Debugf("setting used space to %d\n", used)
	service.status.usedSpace = used
}

// UpdateUsedSpace add/subtract used space for a storagenode
func (service *Service) UpdateUsedSpace(used int64) {
	service.status.sMutex.Lock()
	defer service.status.sMutex.Unlock()
	service.log.Sugar().Debugf("updating used space with %d\n", used)
	service.status.usedSpace += used
}

// AvailableSpace returns available disk space for upload
func (service *Service) AvailableSpace(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	usedSpace := service.GetUsedSpace()
	available := service.allocatedDiskSpace - usedSpace
	service.log.Sugar().Debugf("available space is %d\n", available)
	return available, nil
}

// AvailableBandwidth returns available bandwidth for upload/download
func (service *Service) AvailableBandwidth(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	used := service.GetUsedBandwidth()
	available := service.allocatedBandwidth - used
	service.log.Sugar().Debugf("available bandwidth is %d\n", available)
	return available, nil
}

// Close stops the monitor service.
func (service *Service) Close() (err error) {
	service.Loop.Close()
	return nil
}

func (service *Service) updateNodeInformation(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	usedSpace := service.GetUsedSpace()
	usedBandwidth := service.GetUsedBandwidth()

	service.routingTable.UpdateSelf(&pb.NodeCapacity{
		FreeBandwidth: service.allocatedBandwidth - usedBandwidth,
		FreeDisk:      service.allocatedDiskSpace - usedSpace,
	})

	return nil
}

func (service *Service) usedSpace(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	usedSpace, err := service.pieceInfo.SpaceUsed(ctx)
	if err != nil {
		return 0, err
	}
	return usedSpace, nil
}

func (service *Service) usedBandwidth(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)
	usage, err := bandwidth.TotalMonthlySummary(ctx, service.usageDB)
	if err != nil {
		return 0, err
	}
	return usage.Total(), nil
}
