// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package backofficeui

import (
	"embed"
	"fmt"
	"io/fs"

	backoffice "storj.io/storj/satellite/admin/back-office"
)

//go:embed build/*
var assets embed.FS

func init() {
	build, err := fs.Sub(assets, "build")
	if err != nil {
		panic(fmt.Errorf("invalid embedding: %w", err))
	}

	backoffice.Assets = build
}
