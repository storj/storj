// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	cmd := exec.Command("git", "ls-files", ".", "--others")

	out, err := cmd.Output()
	if err != nil {
		os.Exit(1)
	}

	if strings.TrimSpace(string(out)) != "" {
		fmt.Println("Files left-over after running tests:")
		fmt.Println(string(out))
		os.Exit(1)
	}
}
