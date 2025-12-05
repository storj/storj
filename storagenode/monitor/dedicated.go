// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package monitor

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/storagenode/blobstore/filestore"
)

// DedicatedDisk is a simplified disk checker for the case when disk is dedicated to the storagenode.
type DedicatedDisk struct {
	log              *zap.Logger
	info             *filestore.DirSpaceInfo
	minimumDiskSpace int64
	reservedBytes    int64
}

var _ SpaceReport = (*DedicatedDisk)(nil)

// NewDedicatedDisk creates a new DedicatedDisk.
func NewDedicatedDisk(log *zap.Logger, dir string, minimumDiskSpace, reservedBytes int64) *DedicatedDisk {
	return &DedicatedDisk{
		log:              log,
		info:             filestore.NewDirSpaceInfo(dir),
		minimumDiskSpace: minimumDiskSpace,
		reservedBytes:    reservedBytes,
	}
}

// PreFlightCheck implements SpaceReport interface.
func (d *DedicatedDisk) PreFlightCheck(ctx context.Context) error {
	if d.reservedBytes < 100_000_000 {
		return Error.New("reserved disk space is too low. Minimum is 100 MB")
	}

	status, err := d.info.AvailableSpace(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	// Ensure the disk is at least 500GB in size, which is our current minimum required to be an operator
	if status.AvailableSpace-d.reservedBytes < d.minimumDiskSpace {
		d.log.Error("Total disk space (minus reserved bytes) is less than required minimum", zap.Int64("bytes", d.minimumDiskSpace))
		return Error.New("disk space requirement not met")
	}
	return nil
}

// AvailableSpace implements SpaceReport interface.
func (d *DedicatedDisk) AvailableSpace(ctx context.Context) (_ int64, err error) {
	status, err := d.info.AvailableSpace(ctx)
	if err != nil {
		return 0, errs.Wrap(err)
	}

	availableBytes := status.AvailableSpace - d.reservedBytes
	if availableBytes < 0 {
		availableBytes = 0
	}

	mon.IntVal("allocated_space").Observe(status.TotalSpace - d.reservedBytes)
	mon.IntVal("used_space").Observe(status.TotalSpace - status.AvailableSpace)
	mon.IntVal("available_space").Observe(availableBytes)

	return availableBytes, nil
}

// DiskSpace implements SpaceReport interface.
func (d *DedicatedDisk) DiskSpace(ctx context.Context) (_ DiskSpace, err error) {
	defer mon.Task()(&ctx)(&err)

	storageStatus, err := d.info.AvailableSpace(ctx)
	if err != nil {
		return DiskSpace{}, Error.Wrap(err)
	}

	overused := int64(0)

	availableBytes := storageStatus.AvailableSpace - d.reservedBytes
	if availableBytes < 0 {
		availableBytes = 0
	}

	usedSpace := storageStatus.TotalSpace - storageStatus.AvailableSpace
	if usedSpace < 0 {
		usedSpace = 0
	}
	return DiskSpace{
		Total:     storageStatus.TotalSpace,
		Allocated: storageStatus.TotalSpace,
		Free:      storageStatus.AvailableSpace,
		Available: availableBytes,
		Overused:  overused,
		Used:      usedSpace,
		Reserved:  d.reservedBytes,
	}, nil
}
