// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !unix && !windows

package hashstore

import "os"

const flockSupported = false

func flock(fh *os.File) error { return nil }
