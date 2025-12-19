// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package legacyui

import (
	"embed"
	"fmt"
	"io/fs"

	legacyAdmin "storj.io/storj/satellite/admin/legacy"
)

//go:embed build/*
var assets embed.FS

func init() {
	build, err := fs.Sub(assets, "build")
	if err != nil {
		panic(fmt.Errorf("invalid embedding: %w", err))
	}

	legacyAdmin.Assets = build
}
