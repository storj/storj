// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux

package platform

import (
	"fmt"
	"os"
	"syscall"
)

// GetDiskInfo returns disk space information for the given directory.
// Both the available space and disk identity are derived from the same open
// file descriptor, avoiding TOCTOU races.
func GetDiskInfo(dir string) (DiskInfo, error) {
	fh, err := os.Open(dir)
	if err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}
	defer func() { _ = fh.Close() }()

	fd := int(fh.Fd())

	var statfs syscall.Statfs_t
	if err := syscall.Fstatfs(fd, &statfs); err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}
	if statfs.Bsize <= 0 {
		return DiskInfo{}, Error.New("invalid block size")
	}

	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		return DiskInfo{}, Error.Wrap(err)
	}

	return DiskInfo{
		AvailableSpace: statfs.Bavail * uint64(statfs.Bsize),
		DiskID:         fmt.Sprintf("%d", stat.Dev),
	}, nil
}
