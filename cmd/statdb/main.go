// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/statdb"
)

func main() {
	err := process.Main(process.ConfigEnvironment, &statdb.Service{})
	if err != nil {
		log.Fatal(err)
	}
}
