// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package pieces

import (
	"os"
	"syscall"
)

func openHourFile(fileName string) (*os.File, error) {
	return os.OpenFile(fileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY|syscall.O_CLOEXEC, 0o644)
}
