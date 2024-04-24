// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package filestore

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"storj.io/storj/storagenode/blobstore"
)

func diskInfoFromPath(path string) (info blobstore.DiskInfo, err error) {
	var stat unix.Statfs_t
	err = unix.Statfs(path, &stat)
	if err != nil {
		return blobstore.DiskInfo{TotalSpace: -1, AvailableSpace: -1}, err
	}

	// the Bsize size depends on the OS and unconvert gives a false-positive
	reservedBlocks := int64(stat.Bfree) - int64(stat.Bavail)
	totalSpace := (int64(stat.Blocks) - reservedBlocks) * int64(stat.Bsize) //nolint: unconvert
	availableSpace := int64(stat.Bavail) * int64(stat.Bsize)                //nolint: unconvert
	filesystemID := fmt.Sprintf("%08x%08x", stat.Fsid.Val[0], stat.Fsid.Val[1])

	return blobstore.DiskInfo{
		ID:             filesystemID,
		TotalSpace:     totalSpace,
		AvailableSpace: availableSpace,
	}, nil
}

// rename renames oldpath to newpath.
func rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// openFileReadOnly opens the file with read only.
func openFileReadOnly(path string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY, perm)
}
