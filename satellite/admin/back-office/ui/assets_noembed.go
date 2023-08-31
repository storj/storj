// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build noembed
// +build noembed

package backofficeui

import "io/fs"

// Assets contains either the built admin/back-office/ui or it is empty.
var Assets fs.FS = emptyFS{}

// emptyFS implements an empty filesystem
type emptyFS struct{}

// Open implements fs.FS method.
func (emptyFS) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }
