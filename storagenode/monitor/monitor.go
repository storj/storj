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
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/contact"
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
	// Used includes all the used bytes
	Used int64
	// UsedForPieces is the amount of disk space used for pieces, in bytes.
	UsedForPieces int64
	// UsedForTrash is the amount of disk space used for trash, in bytes.
	UsedForTrash int64
	// UsedReclaimable is the amount of disk space which is used, but can be deleted with the next compaction.
	UsedReclaimable int64
	// Free is the actual amount of free space on the whole disk, not just allocated disk space, in bytes.
	Free int64
	// Available is the amount of free space on the allocated disk space, in bytes.
	Available int64
	// Overused is the amount of disk space overused by the storage node, in bytes.
	Overused int64
	// Reserved is part of the allocated space, but always should be free.
	Reserved int64
}

// Config defines parameters for storage node disk and bandwidth usage monitoring.
type Config struct {
	Interval                  time.Duration `help:"how frequently to report storage stats to the satellite" default:"1h0m0s"`
	VerifyDirReadableInterval time.Duration `help:"how frequently to verify the location and readability of the storage directory" releaseDefault:"1m" devDefault:"30s"`
	VerifyDirWritableInterval time.Duration `help:"how frequently to verify writability of storage directory" releaseDefault:"5m" devDefault:"30s"`
	VerifyDirReadableTimeout  time.Duration `help:"how long to wait for a storage directory readability verification to complete" releaseDefault:"1m" devDefault:"10s"`
	VerifyDirWritableTimeout  time.Duration `help:"how long to wait for a storage directory writability verification to complete" releaseDefault:"1m" devDefault:"10s"`
	VerifyDirWarnOnly         bool          `help:"if the storage directory verification check fails, log a warning instead of killing the node" default:"false"`
	MinimumDiskSpace          memory.Size   `help:"how much disk space a node at minimum has to advertise" default:"500GB"`
	MinimumBandwidth          memory.Size   `help:"how much bandwidth a node at minimum has to advertise (deprecated)" default:"0TB"`
	NotifyLowDiskCooldown     time.Duration `help:"minimum length of time between capacity reports" default:"10m" hidden:"true"`
	DedicatedDisk             bool          `help:"(EXPERIMENTAL) option to dedicate full disk to the storagenode. Allocated space won't be used, some UI / monitoring features will break." default:"false" experimental:"true" hidden:"true"`
	ReservedBytes             memory.Size   `help:"(EXPERIMENTAL) Number bytes to reserve on the disk in case of dedicated disk" default:"300GB" devDefault:"1MB" experimental:"true" hidden:"true"`
}

// DiskVerification is an interface for verifying disk storage healthiness during startup.
type DiskVerification interface {
	VerifyStorageDirWithTimeout(ctx context.Context, id storj.NodeID, timeout time.Duration) error

	CheckWritabilityWithTimeout(ctx context.Context, timeout time.Duration) error
}

// Service which monitors disk usage.
//
// architecture: Service
type Service struct {
	log                   *zap.Logger
	contact               *contact.Service
	cooldown              *sync2.Cooldown
	Loop                  *sync2.Cycle
	VerifyDirReadableLoop *sync2.Cycle
	VerifyDirWritableLoop *sync2.Cycle
	Config                Config
	spaceReport           SpaceReport
	verifier              DiskVerification
	checkInTimeout        time.Duration
}

// NewService creates a new storage node monitoring service.
func NewService(log *zap.Logger, verifier DiskVerification, contact *contact.Service, spaceReport SpaceReport, config Config, checkInTimeout time.Duration) *Service {
	return &Service{
		log:                   log,
		contact:               contact,
		cooldown:              sync2.NewCooldown(config.NotifyLowDiskCooldown),
		Loop:                  sync2.NewCycle(config.Interval),
		VerifyDirReadableLoop: sync2.NewCycle(config.VerifyDirReadableInterval),
		VerifyDirWritableLoop: sync2.NewCycle(config.VerifyDirWritableInterval),
		Config:                config,
		verifier:              verifier,
		spaceReport:           spaceReport,
		checkInTimeout:        checkInTimeout,
	}
}

// Run runs monitor service.
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)
	if service.verifier != nil {
		group.Go(func() error {
			return service.VerifyDirReadableLoop.Run(ctx, service.verifyStorageDir)
		})
		group.Go(func() error {
			return service.VerifyDirWritableLoop.Run(ctx, service.verifyWritability)
		})
	}
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

		err = service.contact.PingSatellites(ctx, service.Config.NotifyLowDiskCooldown, service.checkInTimeout)
		if err != nil {
			service.log.Error("error notifying satellites: ", zap.Error(err))
		}
		return nil
	})

	return group.Wait()
}

func (service *Service) verifyStorageDir(ctx context.Context) error {
	startTime := time.Now()
	timeout := service.Config.VerifyDirReadableTimeout
	err := service.verifier.VerifyStorageDirWithTimeout(ctx, service.contact.Local().ID, timeout)
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
	service.log.Debug("readability check done", zap.Duration("Duration", duration))
	mon.DurationVal("readability_check").Observe(duration)
	return nil
}

func (service *Service) verifyWritability(ctx context.Context) error {
	timeout := service.Config.VerifyDirWritableTimeout
	startTime := time.Now()
	err := service.verifier.CheckWritabilityWithTimeout(ctx, timeout)
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
	service.log.Debug("writability check done", zap.Duration("Duration", duration))
	mon.DurationVal("writability_check").Observe(duration)
	return nil
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

	freeSpace, err := service.spaceReport.AvailableSpace(ctx)
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
	return service.spaceReport.AvailableSpace(ctx)
}

// DiskSpace returns consolidated disk space state info.
func (service *Service) DiskSpace(ctx context.Context) (_ DiskSpace, err error) {
	return service.spaceReport.DiskSpace(ctx)
}

// isLowerThanAllocated checks if the disk space is lower than allocated.
func isLowerThanAllocated(actual, allocated int64) bool {
	return actual > 0 && actual < allocated
}
