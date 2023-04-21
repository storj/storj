// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build unix

package filestore

import (
	"errors"
	"os"
	"syscall"
)

func isLowLevelCorruptionError(err error) bool {
	var perr *os.PathError
	if errors.As(err, &perr) && perr.Op == "lstat" {
		return true
	}
	var errnoErr syscall.Errno
	if errors.As(err, &errnoErr) {
		switch errnoErr {
		case syscall.EBADMSG, syscall.EIO:
			return true
		}
	}
	return false
}
