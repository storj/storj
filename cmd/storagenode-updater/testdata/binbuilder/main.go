// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"strconv"
)

var (
	exitCode string
	version  string
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("Version: %s\n", version)
	}

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
	os.Exit(code)
}
