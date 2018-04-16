// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"log"
	"os"

	"storj.io/storj/internal/app/cli"
)

func main() {
	err := cli.New().Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
