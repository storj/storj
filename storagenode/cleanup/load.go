// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux

package cleanup

import (
	"runtime"
)

func getLoad() (float64, error) {
	return float64(runtime.NumCPU()), nil
}
