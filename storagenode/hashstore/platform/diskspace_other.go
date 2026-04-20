// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux && !windows

package platform

// GetDiskInfo returns disk space information for the given directory.
func GetDiskInfo(dir string) (DiskInfo, error) {
	return DiskInfo{}, Error.New("GetDiskInfo not supported on this platform")
}
