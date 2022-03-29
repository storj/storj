// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build noembed
// +build noembed

package adminui

import "io/fs"

// Assets contains either the built admin/ui or it is empty.
var Assets fs.FS = emptyFS{}

// emptyFS implements an empty filesystem
type emptyFS struct{}

// Open implements fs.FS method.
func (emptyFS) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }
