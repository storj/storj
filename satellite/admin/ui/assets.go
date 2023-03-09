// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !noembed
// +build !noembed

package adminui

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed build/*
var assets embed.FS

// Assets contains either the built admin/ui or it is empty.
var Assets = func() fs.FS {
	build, err := fs.Sub(assets, "build")
	if err != nil {
		panic(fmt.Errorf("invalid embedding: %w", err))
	}
	return build
}()
