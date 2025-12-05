// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux

package platform

import "os"

// Fallocate preallocates space for a file. It is a no-op on platforms that do
// not support it.
func Fallocate(fh *os.File, size int64) error { return nil }
