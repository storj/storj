// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/storagenode/pieces"
)

// HashStoreBackend is an interface describing the methods needed by SharedDisk
// to correctly compute the space usage of the hash store.
type HashStoreBackend interface {
	SpaceUsage() SpaceUsage
}

// SpaceUsage describes the amount of space used by a PieceBackend.
type SpaceUsage struct {
	UsedTotal       int64 // total space used including metadata and unreferenced data
	UsedForPieces   int64 // total space used by live pieces
	UsedForTrash    int64 // total space used by trash pieces
	UsedForMetadata int64 // total space used by metadata (hash tables and stuff)
	UsedReclaimable int64 // space used that can be reclaimed (e.g., unreferenced data)
}

// SharedDisk is the default way to check disk space (using usage-space walker).
type SharedDisk struct {
	store              *pieces.Store
	hashStore          HashStoreBackend
	allocatedDiskSpace int64
	log                *zap.Logger
	minimumDiskSpace   int64
}

var _ SpaceReport = (*SharedDisk)(nil)

// NewSharedDisk creates a new SharedDisk.
func NewSharedDisk(log *zap.Logger, store *pieces.Store, hashStore HashStoreBackend, minimumDiskSpace, allocatedDiskSpace int64) *SharedDisk {
	return &SharedDisk{
		log:                log,
		store:              store,
		hashStore:          hashStore,
		allocatedDiskSpace: allocatedDiskSpace,
		minimumDiskSpace:   minimumDiskSpace,
	}
}

// PreFlightCheck checks if the disk is ready to use.
func (s *SharedDisk) PreFlightCheck(ctx context.Context) error {
	// get the disk space details
	// The returned path ends in a slash only if it represents a root directory, such as "/" on Unix or `C:\` on Windows.
	storageStatus, err := s.store.StorageStatus(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	freeDiskSpace := storageStatus.DiskFree

	totalUsed, err := s.store.SpaceUsedForPiecesAndTrash(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	// check your hard drive is big enough
	// first time setup as a piece node server
	if totalUsed == 0 && freeDiskSpace < s.allocatedDiskSpace {
		s.allocatedDiskSpace = freeDiskSpace
		s.log.Warn("Disk space is less than requested. Allocated space is", zap.Int64("bytes", s.allocatedDiskSpace))
	}

	// on restarting the Piece node server, assuming already been working as a node
	// used above the allocated space, user changed the allocation space setting
	// before restarting
	if totalUsed >= s.allocatedDiskSpace {
		s.log.Warn("Used more space than allocated. Allocated space is", zap.Int64("bytes", s.allocatedDiskSpace))
	}

	// the available disk space is less than remaining allocated space,
	// due to change of setting before restarting
	if freeDiskSpace < s.allocatedDiskSpace-totalUsed {
		s.allocatedDiskSpace = freeDiskSpace + totalUsed
		s.log.Warn("Disk space is less than requested. Allocated space is", zap.Int64("bytes", s.allocatedDiskSpace))
	}

	// Ensure the disk is at least 500GB in size, which is our current minimum required to be an operator
	if s.allocatedDiskSpace < s.minimumDiskSpace {
		s.log.Error("Total disk space is less than required minimum", zap.Int64("bytes", s.minimumDiskSpace))
		return Error.New("disk space requirement not met")
	}
	return nil
}

// AvailableSpace returns available disk space for upload.
func (s *SharedDisk) AvailableSpace(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	usedSpace, err := s.store.SpaceUsedForPiecesAndTrash(ctx)
	if err != nil {
		return 0, err
	}
	hashSpaceUsage := s.hashStore.SpaceUsage()

	usedSpace += hashSpaceUsage.UsedTotal

	diskStatus, err := s.store.StorageStatus(ctx)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	allocated := s.allocatedDiskSpace
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
func (s *SharedDisk) DiskSpace(ctx context.Context) (_ DiskSpace, err error) {
	defer mon.Task()(&ctx)(&err)

	usedForPieces, _, err := s.store.SpaceUsedForPieces(ctx)
	if err != nil {
		return DiskSpace{}, Error.Wrap(err)
	}
	usedForTrash, err := s.store.SpaceUsedForTrash(ctx)
	if err != nil {
		return DiskSpace{}, Error.Wrap(err)
	}
	hashSpaceUsage := s.hashStore.SpaceUsage()

	storageStatus, err := s.store.StorageStatus(ctx)
	if err != nil {
		return DiskSpace{}, Error.Wrap(err)
	}

	overused := int64(0)

	allocated := s.allocatedDiskSpace
	if isLowerThanAllocated(storageStatus.DiskTotal, allocated) {
		allocated = storageStatus.DiskTotal
	}

	available := allocated - (usedForPieces + usedForTrash) - hashSpaceUsage.UsedTotal
	if available < 0 {
		overused = -available
		available = 0
	}
	if storageStatus.DiskFree < available {
		available = storageStatus.DiskFree
	}

	return DiskSpace{
		Total:         storageStatus.DiskTotal,
		Allocated:     allocated,
		UsedForPieces: usedForPieces + hashSpaceUsage.UsedForPieces,
		UsedForTrash:  usedForTrash + hashSpaceUsage.UsedForTrash,
		Free:          storageStatus.DiskFree,
		Available:     available,
		Overused:      overused,
		Used:          usedForPieces + usedForTrash + hashSpaceUsage.UsedTotal,
	}, nil
}
