// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
)

var (
	exitCode string
	version  string
	ctx      = context.Background()
)

func main() {
	var (
		code int
		err  error
	)
	if exitCode == "" {
		code = 0
	} else {
		code, err = strconv.Atoi(exitCode)
		if err != nil {
			panic(err)
		}
	}

	command := os.Args[1]
	if len(os.Args) > 1 && command == "version" {
		fmt.Printf("Version: %s\n", version)
	}
	os.Exit(code)
}
