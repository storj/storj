// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"

	"storj.io/storj/pkg/netstate"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/process"
)

func main() {
	err := process.Main(
		overlay.NewService(nil, nil),
		netstate.NewService(nil, nil),
	)
	if err != nil {
		log.Fatal(err)
	}
}
