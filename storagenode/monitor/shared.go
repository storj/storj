// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/storagenode/blobstore/filestore"
)

// HashStoreBackend is an interface describing the methods needed by SharedDisk
// to correctly compute the space usage of the hash store.
type HashStoreBackend interface {
	SpaceUsage() SpaceUsage
	LogsPath() string
}

// SpaceUsage describes the amount of space used by a PieceBackend.
type SpaceUsage struct {
	UsedTotal       int64 // total space used including metadata and unreferenced data
	UsedForPieces   int64 // total space used by live pieces
	UsedForTrash    int64 // total space used by trash pieces
	UsedForMetadata int64 // total space used by metadata (hash tables and stuff)
	UsedReclaimable int64 // space used that can be reclaimed (e.g., unreferenced data)
	Reserved        int64 // space that should always be free (for example: for temp files during compaction)
}

// StorageStatus contains information about the disk store is using.
type StorageStatus struct {
	// DiskTotal is the actual disk size (not just the allocated disk space), in bytes.
	DiskTotal int64
	DiskUsed  int64
	// DiskFree is the actual amount of free space on the whole disk, not just allocated disk space, in bytes.
	DiskFree int64
}

// PieceStoreSpaceUsage is an interface describing the methods needed by SharedDisk
// to correctly compute the space usage of the piece store.
type PieceStoreSpaceUsage interface {
	StorageStatus(ctx context.Context) (StorageStatus, error)
	SpaceUsedForPieces(ctx context.Context) (piecesTotal int64, piecesContentSize int64, err error)
	SpaceUsedForTrash(ctx context.Context) (int64, error)
	SpaceUsedForPiecesAndTrash(ctx context.Context) (int64, error)
}

// SharedDisk is the default way to check disk space (using usage-space walker).
type SharedDisk struct {
	store              PieceStoreSpaceUsage
	hashStore          HashStoreBackend
	allocatedDiskSpace int64
	log                *zap.Logger
	minimumDiskSpace   int64
	dir                *filestore.DirSpaceInfo
}

var _ SpaceReport = (*SharedDisk)(nil)

// NewSharedDisk creates a new SharedDisk.
func NewSharedDisk(ctx context.Context, log *zap.Logger, store PieceStoreSpaceUsage, hashStore HashStoreBackend, minimumDiskSpace, allocatedDiskSpace int64) (*SharedDisk, error) {
	s := &SharedDisk{
		log:                log,
		dir:                filestore.NewDirSpaceInfo(hashStore.LogsPath()),
		store:              store,
		hashStore:          hashStore,
		allocatedDiskSpace: allocatedDiskSpace,
		minimumDiskSpace:   minimumDiskSpace,
	}
	return s, s.PreFlightCheck(ctx)
}

// PreFlightCheck checks if the disk is ready to use.
func (s *SharedDisk) PreFlightCheck(ctx context.Context) error {
	if s.store == nil {
		return nil
	}

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

// DiskSpace returns consolidated disk space state info.
func (s *SharedDisk) DiskSpace(ctx context.Context) (_ DiskSpace, err error) {
	defer mon.Task()(&ctx)(&err)

	var usedForPieces, usedForTrash int64
	var storageStatus StorageStatus

	if s.store != nil {
		usedForPieces, _, err = s.store.SpaceUsedForPieces(ctx)
		if err != nil {
			return DiskSpace{}, Error.Wrap(err)
		}
		usedForTrash, err = s.store.SpaceUsedForTrash(ctx)
		if err != nil {
			return DiskSpace{}, Error.Wrap(err)
		}
		storageStatus, err = s.store.StorageStatus(ctx)
		if err != nil {
			return DiskSpace{}, Error.Wrap(err)
		}
	} else {
		as, err := s.dir.AvailableSpace(ctx)
		if err != nil {
			s.log.Warn("unable to get disk space info, using zeros", zap.Error(err), zap.String("dir", s.hashStore.LogsPath()))
		} else {
			storageStatus = StorageStatus{
				DiskTotal: as.TotalSpace,
				DiskFree:  as.AvailableSpace,
			}
		}
	}

	hashSpaceUsage := s.hashStore.SpaceUsage()

	overused := int64(0)

	allocated := s.allocatedDiskSpace
	if isLowerThanAllocated(storageStatus.DiskTotal, allocated) {
		allocated = storageStatus.DiskTotal
	}

	available := allocated - (usedForPieces + usedForTrash) - hashSpaceUsage.UsedTotal - hashSpaceUsage.Reserved
	if available < 0 {
		overused = -available
		available = 0
	}
	if s.store != nil && storageStatus.DiskFree < available {
		available = storageStatus.DiskFree
	}

	diskSpace := DiskSpace{
		Total:           storageStatus.DiskTotal,
		Allocated:       allocated,
		UsedForPieces:   usedForPieces + hashSpaceUsage.UsedForPieces,
		UsedForTrash:    usedForTrash + hashSpaceUsage.UsedForTrash,
		Free:            storageStatus.DiskFree,
		Available:       available,
		Overused:        overused,
		Used:            usedForPieces + usedForTrash + hashSpaceUsage.UsedTotal,
		UsedReclaimable: hashSpaceUsage.UsedReclaimable,
		Reserved:        hashSpaceUsage.Reserved,
	}

	mon.IntVal("allocated_space").Observe(diskSpace.Allocated)
	mon.IntVal("used_space").Observe(diskSpace.Used)
	mon.IntVal("available_space").Observe(diskSpace.Available)
	mon.IntVal("reserved_space").Observe(diskSpace.Reserved)

	return diskSpace, nil
}
