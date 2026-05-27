// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

// gen converts HTML email templates to plain-text template files.
// Run via: go generate ./satellite/mailservice/
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"storj.io/storj/satellite/mailservice/htmltext"
)

func main() {
	dir := flag.String("dir", "../../web/satellite/static/emails", "path to email templates directory")
	flag.Parse()

	matches, err := filepath.Glob(filepath.Join(*dir, "*.html"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob: %v\n", err)
		os.Exit(1)
	}
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "no HTML templates found in %s\n", *dir)
		os.Exit(1)
	}

	for _, src := range matches {
		f, err := os.Open(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open %s: %v\n", src, err)
			os.Exit(1)
		}
		text := htmltext.Convert(f)
		f.Close()

		dst := strings.TrimSuffix(src, ".html") + ".txt"
		if err := os.WriteFile(dst, []byte(text), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", dst, err)
			os.Exit(1)
		}
		fmt.Println(dst)
	}
}
