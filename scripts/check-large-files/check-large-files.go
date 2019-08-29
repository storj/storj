// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	KB = 1e3
	MB = 1e6

	fileSizeLimit = 600 * KB
)

func main() {
	basePath := ".."
	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}

	var failed int

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			failed++
			fmt.Println(err)
			return nil
		}
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules") {
			return filepath.SkipDir
		}

		size := info.Size()
		if size > fileSizeLimit {
			failed++
			if rel, err := filepath.Rel(basePath, path); err == nil {
				path = rel
			}
			fmt.Printf("%v (%s)\n", path, formatSize(size))
		}

		return nil
	})
	if err != nil {
		fmt.Println(err)
	}

	if failed > 0 {
		fmt.Printf("some files were over size limit %s\n", formatSize(fileSizeLimit))
		os.Exit(1)
	}
}

func formatSize(size int64) string {
	switch fsize := float64(size); {
	case fsize >= MB*2/3:
		return fmt.Sprintf("%.1f MB", fsize/MB)
	case fsize >= KB*2/3:
		return fmt.Sprintf("%.1f KB", fsize/KB)
	default:
		return strconv.FormatInt(size, 10) + " B"
	}
}
