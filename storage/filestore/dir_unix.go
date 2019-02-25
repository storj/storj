// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !windows

package filestore

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func isBusy(err error) bool {
	err = underlyingError(err)
	return err == unix.EBUSY
}

func diskInfoFromPath(path string) (info DiskInfo, err error) {
	var stat unix.Statfs_t
	err = unix.Statfs(path, &stat)
	if err != nil {
		return DiskInfo{"", -1}, err
	}

	// the Bsize size depends on the OS and unconvert gives a false-positive
	availableSpace := int64(stat.Bavail) * int64(stat.Bsize) //nolint
	filesystemID := fmt.Sprintf("%08x%08x", stat.Fsid.Val[0], stat.Fsid.Val[1])

	return DiskInfo{filesystemID, availableSpace}, nil
}
