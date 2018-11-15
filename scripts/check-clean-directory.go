// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var allowedFiles = []string{
	"Jenkinsfile", "LICENSE", "Makefile", "Dockerfile",
}

var allowedExts = []string{
	// necessary for testing
	".coverprofile",
	// go files
	".go", ".proto", ".sum", ".mod", ".dbx", ".sql",
	// scripts
	".ps1", ".sh",
	// configs
	".gitignore", ".clabot", ".dockerignore", ".yaml", ".yml",
	// web
	".html", ".vue", ".ts", ".js", ".json", ".snap",
	// documentation, binaries
	".md", ".svg", ".png", ".ttf", ".ico",
}

func contains(xs []string, p string) bool {
	for _, x := range xs {
		if strings.EqualFold(x, p) {
			return true
		}
	}
	return false
}

func main() {
	var failed int

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}

		allowedFile := contains(allowedFiles, filepath.Base(path))
		allowedExt := contains(allowedExts, filepath.Ext(path))

		if !(allowedFile || allowedExt) {
			failed++
			fmt.Printf("unexpected file %v: %v\n", path, err)
			return nil
		}

		return nil
	})
	if err != nil {
		fmt.Println(err)
	}

	if failed > 0 {
		os.Exit(1)
	}
}
