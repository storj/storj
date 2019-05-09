// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

// check-peer-constraints checks that none of the core packages import peers directly.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

var race = flag.Bool("race", false, "load with race tag")
var fail = flag.Bool("fail", true, "fail on violation")

func main() {
	flag.Parse()

	var exitcode int

	peers, err := load(
		"storj.io/storj/satellite/...",
		"storj.io/storj/storagenode/...",
		"storj.io/storj/bootstrap/...",
	)
	if err != nil {
		fmt.Printf("failed to load peers: %v\n", err)
		os.Exit(1)
	}

	for _, source := range []string{
		"storj.io/storj/pkg/...",
		"storj.io/storj/lib/...",
		"storj.io/storj/uplink/...",
	} {
		sources, err := load(source)
		if err != nil {
			fmt.Printf("failed to load %q: %v\n", source, err)
			os.Exit(1)
		}

		if links(sources, peers) {
			exitcode = 1
		}
	}

	if *fail {
		os.Exit(exitcode)
	}
}

func load(names ...string) ([]*packages.Package, error) {
	var buildFlags []string
	if *race {
		buildFlags = append(buildFlags, "-race")
	}

	return packages.Load(&packages.Config{
		Mode:       packages.LoadImports,
		Env:        os.Environ(),
		BuildFlags: buildFlags,
		Tests:      false,
	}, names...)
}

func links(source, destination []*packages.Package) bool {
	targets := map[string]bool{}
	for _, dst := range destination {
		targets[dst.ID] = true
	}

	links := false
	visited := map[string]bool{}

	var visit func(pkg *packages.Package, path []*packages.Package)
	visit = func(pkg *packages.Package, path []*packages.Package) {
		for id, imp := range pkg.Imports {
			if _, visited := visited[id]; visited {
				continue
			}
			visited[id] = true
			if targets[id] {
				fmt.Printf("import %q\n", pathstr(append(path, pkg, imp)))
				continue
			}
			visit(imp, append(path, pkg))
		}
	}

	for _, pkg := range source {
		visit(pkg, nil)
	}

	return links
}

func pathstr(path []*packages.Package) string {
	ids := []string{}
	for _, pkg := range path {
		ids = append(ids, pkg.ID)
	}

	return strings.Join(ids, " > ")
}
