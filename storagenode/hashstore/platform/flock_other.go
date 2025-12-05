// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !unix && !windows

package platform

import "os"

// FlockSupported is a constant indicating if flock is supported on the platform.
const FlockSupported = false

func flock(fh *os.File) error { return nil }
