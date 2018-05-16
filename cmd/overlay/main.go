// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/process"
)

func main() {
	if err := process.Main(&overlay.Service{}); err != nil {
		log.Fatal(err)
	}
}
