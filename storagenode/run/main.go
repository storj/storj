// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
	_ "storj.io/storj/web/storagenode" // This embeds storagenode assets.
)

// Main is the main entrypoint. Can be called from real `main` package after importing optional modules.
func Main() {
	rootCmd, _ := newRootCmd()

	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
