// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build unix

package hashstore

import (
	"os"
	"syscall"

	"github.com/zeebo/errs"
)

const flockSupported = true

func flock(fh *os.File) error {
	return errs.Wrap(syscall.Flock(
		int(fh.Fd()),
		syscall.LOCK_EX|syscall.LOCK_NB,
	))
}
