// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package monitor is responsible for monitoring the disk is well-behaved.
// It checks whether there's sufficient space and whether directories are writable.
package monitor

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/pieces"
)

var (
	mon = monkit.Package()

	// Error is the default error class for piecestore monitor errors.
	Error = errs.Class("piecestore monitor")
)

// DiskSpace consolidates monitored disk space statistics.
type DiskSpace struct {
	// Allocated is the amount of disk space allocated to the storage node, in bytes.
	Allocated int64
	// Total is the total amount of disk space on the whole disk, not just allocated disk space, in bytes.
	Total int64
	// UsedForPieces is the amount of disk space used for pieces, in bytes.
	UsedForPieces int64
	// UsedForTrash is the amount of disk space used for trash, in bytes.
	UsedForTrash int64
	// Free is the actual amount of free space on the whole disk, not just allocated disk space, in bytes.
	Free int64
	// Available is the amount of free space on the allocated disk space, in bytes.
	Available int64
	// Overused is the amount of disk space overused by the storage node, in bytes.
	Overused int64
}

// Config defines parameters for storage node disk and bandwidth usage monitoring.
type Config struct {
	Interval                  time.Duration `help:"how frequently Kademlia bucket should be refreshed with node stats" default:"1h0m0s"`
	VerifyDirReadableInterval time.Duration `help:"how frequently to verify the location and readability of the storage directory" releaseDefault:"1m" devDefault:"30s"`
	VerifyDirWritableInterval time.Duration `help:"how frequently to verify writability of storage directory" releaseDefault:"5m" devDefault:"30s"`
	VerifyDirReadableTimeout  time.Duration `help:"how long to wait for a storage directory readability verification to complete" releaseDefault:"1m" devDefault:"10s"`
	VerifyDirWritableTimeout  time.Duration `help:"how long to wait for a storage directory writability verification to complete" releaseDefault:"1m" devDefault:"10s"`
	VerifyDirWarnOnly         bool          `help:"if the storage directory verification check fails, log a warning instead of killing the node" default:"false"`
	MinimumDiskSpace          memory.Size   `help:"how much disk space a node at minimum has to advertise" default:"500GB"`
	MinimumBandwidth          memory.Size   `help:"how much bandwidth a node at minimum has to advertise (deprecated)" default:"0TB"`
	NotifyLowDiskCooldown     time.Duration `help:"minimum length of time between capacity reports" default:"10m" hidden:"true"`
}

// Service which monitors disk usage.
//
// architecture: Service
type Service struct {
	log                   *zap.Logger
	store                 *pieces.Store
	contact               *contact.Service
	allocatedDiskSpace    int64
	cooldown              *sync2.Cooldown
	Loop                  *sync2.Cycle
	VerifyDirReadableLoop *sync2.Cycle
	VerifyDirWritableLoop *sync2.Cycle
	Config                Config
}

// NewService creates a new storage node monitoring service.
func NewService(log *zap.Logger, store *pieces.Store, contact *contact.Service, allocatedDiskSpace int64, interval time.Duration, reportCapacity func(context.Context), config Config) *Service {
	return &Service{
		log:                   log,
		store:                 store,
		contact:               contact,
		allocatedDiskSpace:    allocatedDiskSpace,
		cooldown:              sync2.NewCooldown(config.NotifyLowDiskCooldown),
		Loop:                  sync2.NewCycle(interval),
		VerifyDirReadableLoop: sync2.NewCycle(config.VerifyDirReadableInterval),
		VerifyDirWritableLoop: sync2.NewCycle(config.VerifyDirWritableInterval),
		Config:                config,
	}
}

// Run runs monitor service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// get the disk space details
	// The returned path ends in a slash only if it represents a root directory, such as "/" on Unix or `C:\` on Windows.
	storageStatus, err := service.store.StorageStatus(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	freeDiskSpace := storageStatus.DiskFree

	totalUsed, err := service.store.SpaceUsedForPiecesAndTrash(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	// check your hard drive is big enough
	// first time setup as a piece node server
	if totalUsed == 0 && freeDiskSpace < service.allocatedDiskSpace {
		service.allocatedDiskSpace = freeDiskSpace
		service.log.Warn("Disk space is less than requested. Allocated space is", zap.Int64("bytes", service.allocatedDiskSpace))
	}

	// on restarting the Piece node server, assuming already been working as a node
	// used above the alloacated space, user changed the allocation space setting
	// before restarting
	if totalUsed >= service.allocatedDiskSpace {
		service.log.Warn("Used more space than allocated. Allocated space is", zap.Int64("bytes", service.allocatedDiskSpace))
	}

	// the available disk space is less than remaining allocated space,
	// due to change of setting before restarting
	if freeDiskSpace < service.allocatedDiskSpace-totalUsed {
		service.allocatedDiskSpace = freeDiskSpace + totalUsed
		service.log.Warn("Disk space is less than requested. Allocated space is", zap.Int64("bytes", service.allocatedDiskSpace))
	}

	// Ensure the disk is at least 500GB in size, which is our current minimum required to be an operator
	if service.allocatedDiskSpace < service.Config.MinimumDiskSpace.Int64() {
		service.log.Error("Total disk space is less than required minimum", zap.Int64("bytes", service.Config.MinimumDiskSpace.Int64()))
		return Error.New("disk space requirement not met")
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		timeout := service.Config.VerifyDirReadableTimeout
		return service.VerifyDirReadableLoop.Run(ctx, func(ctx context.Context) error {
			startTime := time.Now()
			err := service.store.VerifyStorageDirWithTimeout(ctx, service.contact.Local().ID, timeout)
			duration := time.Since(startTime)
			if err != nil {
				if errs2.IsCanceled(err) {
					return nil
				}
				if errs.Is(err, context.DeadlineExceeded) {
					if service.Config.VerifyDirWarnOnly {
						service.log.Error("timed out while verifying readability of storage directory", zap.Duration("timeout", timeout))
						return nil
					}
					return Error.New("timed out after %v while verifying readability of storage directory", timeout)
				}
				if service.Config.VerifyDirWarnOnly {
					service.log.Error("error verifying location and/or readability of storage directory", zap.Error(err))
					return nil
				}
				return Error.New("error verifying location and/or readability of storage directory: %v", err)
			}
			service.log.Info("readability check done", zap.Duration("Duration", duration))
			mon.DurationVal("readability_check").Observe(duration)
			return nil
		})
	})
	group.Go(func() error {
		timeout := service.Config.VerifyDirWritableTimeout
		return service.VerifyDirWritableLoop.Run(ctx, func(ctx context.Context) error {
			startTime := time.Now()
			err := service.store.CheckWritabilityWithTimeout(ctx, timeout)
			duration := time.Since(startTime)
			if err != nil {
				if errs2.IsCanceled(err) {
					return nil
				}
				if errs.Is(err, context.DeadlineExceeded) {
					if service.Config.VerifyDirWarnOnly {
						service.log.Error("timed out while verifying writability of storage directory", zap.Duration("timeout", timeout))
						return nil
					}
					return Error.New("timed out after %v while verifying writability of storage directory", timeout)
				}
				if service.Config.VerifyDirWarnOnly {
					service.log.Error("error verifying writability of storage directory", zap.Error(err))
					return nil
				}
				return Error.New("error verifying writability of storage directory: %v", err)
			}
			service.log.Info("writability check done", zap.Duration("Duration", duration))
			mon.DurationVal("writability_check").Observe(duration)
			return nil
		})
	})
	group.Go(func() error {
		return service.Loop.Run(ctx, func(ctx context.Context) error {
			err := service.updateNodeInformation(ctx)
			if err != nil {
				service.log.Error("error during updating node information: ", zap.Error(err))
			}
			return nil
		})
	})
	service.cooldown.Start(ctx, group, func(ctx context.Context) error {
		err := service.updateNodeInformation(ctx)
		if err != nil {
			service.log.Error("error during updating node information: ", zap.Error(err))
			return nil
		}

		err = service.contact.PingSatellites(ctx, service.Config.NotifyLowDiskCooldown)
		if err != nil {
			service.log.Error("error notifying satellites: ", zap.Error(err))
		}
		return nil
	})

	return group.Wait()
}

// NotifyLowDisk reports disk space to satellites if cooldown timer has expired.
func (service *Service) NotifyLowDisk() {
	service.cooldown.Trigger()
}

// Close stops the monitor service.
func (service *Service) Close() (err error) {
	service.Loop.Close()
	service.cooldown.Close()
	return nil
}

func (service *Service) updateNodeInformation(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	freeSpace, err := service.AvailableSpace(ctx)
	if err != nil {
		return err
	}
	service.contact.UpdateSelf(&pb.NodeCapacity{
		FreeDisk: freeSpace,
	})

	return nil
}

// AvailableSpace returns available disk space for upload.
func (service *Service) AvailableSpace(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	usedSpace, err := service.store.SpaceUsedForPiecesAndTrash(ctx)
	if err != nil {
		return 0, err
	}

	diskStatus, err := service.store.StorageStatus(ctx)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	allocated := service.allocatedDiskSpace
	if isLowerThanAllocated(diskStatus.DiskTotal, allocated) {
		allocated = diskStatus.DiskTotal
	}

	freeSpaceForStorj := allocated - usedSpace
	if diskStatus.DiskFree < freeSpaceForStorj {
		freeSpaceForStorj = diskStatus.DiskFree
	}

	mon.IntVal("allocated_space").Observe(allocated)
	mon.IntVal("used_space").Observe(usedSpace)
	mon.IntVal("available_space").Observe(freeSpaceForStorj)

	return freeSpaceForStorj, nil
}

// DiskSpace returns consolidated disk space state info.
func (service *Service) DiskSpace(ctx context.Context) (_ DiskSpace, err error) {
	defer mon.Task()(&ctx)(&err)

	usedForPieces, _, err := service.store.SpaceUsedForPieces(ctx)
	if err != nil {
		return DiskSpace{}, Error.Wrap(err)
	}
	usedForTrash, err := service.store.SpaceUsedForTrash(ctx)
	if err != nil {
		return DiskSpace{}, Error.Wrap(err)
	}

	storageStatus, err := service.store.StorageStatus(ctx)
	if err != nil {
		return DiskSpace{}, Error.Wrap(err)
	}

	overused := int64(0)

	allocated := service.allocatedDiskSpace
	if isLowerThanAllocated(storageStatus.DiskTotal, allocated) {
		allocated = storageStatus.DiskTotal
	}

	available := allocated - (usedForPieces + usedForTrash)
	if available < 0 {
		overused = -available
	}
	if storageStatus.DiskFree < available {
		available = storageStatus.DiskFree
	}

	return DiskSpace{
		Total:         storageStatus.DiskTotal,
		Allocated:     allocated,
		UsedForPieces: usedForPieces,
		UsedForTrash:  usedForTrash,
		Free:          storageStatus.DiskFree,
		Available:     available,
		Overused:      overused,
	}, nil
}

// isLowerThanAllocated checks if the disk space is lower than allocated.
func isLowerThanAllocated(actual, allocated int64) bool {
	return actual > 0 && actual < allocated
}
