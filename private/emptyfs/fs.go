// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emptyfs

import "io/fs"

// FS implements an empty filesystem.
type FS struct{}

// Open implements fs.FS method.
func (FS) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }
