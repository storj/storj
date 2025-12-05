// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !unix

package filestore

import (
	"errors"
	"os"
	"strings"
)

func isLowLevelCorruptionError(err error) bool {
	// convert to lowercase the perr.Op because Go returns inconsistently
	// "lstat" in Linux and "Lstat" in Windows
	var perr *os.PathError
	if errors.As(err, &perr) && strings.ToLower(perr.Op) == "lstat" {
		return true
	}
	return false
}
