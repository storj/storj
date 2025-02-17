// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import root "storj.io/storj/storagenode/run"

// main is an EXPERIMENTAL entrypoint which doesn't depend on process.
// You should use ./cmd/storagenode for now, except you know what will happen.
func main() {
	root.Main()
}
