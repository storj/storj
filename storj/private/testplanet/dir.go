// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"runtime"
	"strings"
)

var developmentRoot string

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return
	}

	developmentRoot = strings.TrimSuffix(filename, "/private/testplanet/dir.go")
}
