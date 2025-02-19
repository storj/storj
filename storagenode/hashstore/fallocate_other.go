// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux

package hashstore

import "os"

func fallocate(fh *os.File, size int64) error { return nil }
