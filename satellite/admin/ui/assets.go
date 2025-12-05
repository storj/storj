// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package adminui

import (
	"embed"
	"fmt"
	"io/fs"

	"storj.io/storj/satellite/admin"
)

//go:embed build/*
var assets embed.FS

func init() {
	build, err := fs.Sub(assets, "build")
	if err != nil {
		panic(fmt.Errorf("invalid embedding: %w", err))
	}

	admin.Assets = build
}
