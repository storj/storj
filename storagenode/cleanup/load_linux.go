// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux

package cleanup

import (
	"os"
	"strconv"
	"strings"

	"github.com/zeebo/errs"
)

// getLoad returns with the current load.
func getLoad() (float64, error) {
	content, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, errs.Wrap(err)
	}

	fields := strings.Fields(string(content))
	if len(fields) < 2 {
		return 0, errs.Wrap(errs.New("couldn't parse /proc/loadavg, not enough fields"))
	}

	loadavg, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, errs.Wrap(errs.New("couldn't parse /proc/loadavg, not a number"))
	}

	return loadavg, nil
}
