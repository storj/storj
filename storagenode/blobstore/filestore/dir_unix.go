// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package filestore

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"

	"storj.io/storj/storagenode/blobstore"
)

// DiskInfoFromPath returns the disk info for the given path.
func DiskInfoFromPath(path string) (info blobstore.DiskInfo, err error) {
	var stat unix.Statfs_t
	err = unix.Statfs(path, &stat)
	if err != nil {
		return blobstore.DiskInfo{TotalSpace: -1, AvailableSpace: -1}, err
	}

	// the Bsize size depends on the OS and unconvert gives a false-positive
	reservedBlocks := int64(stat.Bfree) - int64(stat.Bavail)
	totalSpace := (int64(stat.Blocks) - reservedBlocks) * int64(stat.Bsize) //nolint: unconvert
	availableSpace := int64(stat.Bavail) * int64(stat.Bsize)                //nolint: unconvert

	return blobstore.DiskInfo{
		TotalSpace:     totalSpace,
		AvailableSpace: availableSpace,
	}, nil
}

// rename renames oldpath to newpath.
func rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// rmDir removes the directory named by path.
func rmDir(path string) error {
	for {
		err := syscall.Rmdir(path)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

// openFileReadOnly opens the file with read only.
func openFileReadOnly(path string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY, perm)
}
