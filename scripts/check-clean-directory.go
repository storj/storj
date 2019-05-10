// Copyright (C) 2019 Storj Labs, Inc.
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
		fmt.Println("Checking left-over files failed.")
		fmt.Println(err)
		os.Exit(1)
	}

	leftover := strings.Split(strings.TrimSpace(string(out)), "\n")
	leftover = ignoreDir(leftover, ".build")

	if len(leftover) != 0 {
		fmt.Println("Files left-over after running tests:")
		for _, file := range leftover {
			fmt.Println(file)
		}
		os.Exit(1)
	}
}

func ignoreDir(files []string, dir string) []string {
	result := files[:0]
	for _, file := range files {
		if file == "" {
			continue
		}
		if strings.HasPrefix(file, dir) {
			continue
		}
		result = append(result, file)
	}
	return result
}
