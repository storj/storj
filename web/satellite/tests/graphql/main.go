// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"

	"storj.io/storj/web/satellite/tests/graphql/endpoints"
)

func main() {
	exitcode := endpoints.Endpoints()
	fmt.Println(exitcode)
	fmt.Println("Introspection Test: Exiting")
	os.Exit(exitcode)
}
