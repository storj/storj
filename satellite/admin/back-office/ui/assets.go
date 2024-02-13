// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package backofficeui

import (
	"embed"
	"fmt"
	"io/fs"

	admin "storj.io/storj/satellite/admin/back-office"
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
