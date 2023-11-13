// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !noembed
// +build !noembed

package backofficeui

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:build/*
var assets embed.FS

// Assets contains either the built admin/back-office/ui or it is empty.
var Assets = func() fs.FS {
	build, err := fs.Sub(assets, "build")
	if err != nil {
		panic(fmt.Errorf("invalid embedding: %w", err))
	}
	return build
}()
