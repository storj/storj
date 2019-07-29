// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

// check-peer-constraints checks that none of the core packages import peers directly.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

var Libraries = []string{
	"storj.io/storj/pkg/...",
	"storj.io/storj/lib/...",
	"storj.io/storj/uplink/...",
	"storj.io/storj/mobile/...",
	"storj.io/storj/storage/...",
}

var Peers = []string{
	"storj.io/storj/satellite/...",
	"storj.io/storj/storagenode/...",
	"storj.io/storj/bootstrap/...",
	"storj.io/storj/versioncontrol/...",
	"storj.io/storj/linksharing/...",
	"storj.io/storj/cmd/...",
}

var race = flag.Bool("race", false, "load with race tag")

func main() {
	flag.Parse()

	var buildFlags []string
	if *race {
		buildFlags = append(buildFlags, "-race")
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode:       packages.LoadImports,
		Env:        os.Environ(),
		BuildFlags: buildFlags,
		Tests:      false,
	}, "storj.io/storj/...")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load pacakges: %v\n", err)
		os.Exit(1)
	}
	pkgs = flatten(pkgs)

	exitcode := 0

	for _, library := range Libraries {
		source := match(pkgs, library)
		for _, peer := range Peers {
			destination := match(pkgs, peer)
			if links(source, destination) {
				fmt.Fprintf(os.Stdout, "%q is importing %q\n", library, peer)
				exitcode = 1
			}
		}
	}

	os.Exit(exitcode)
}

func match(pkgs []*packages.Package, globs ...string) []*packages.Package {
	for i, glob := range globs {
		globs[i] = strings.ReplaceAll(glob, "...", ".*")
	}
	rx := regexp.MustCompile("^(" + strings.Join(globs, "|") + ")$")

	var rs []*packages.Package
	for _, pkg := range pkgs {
		if rx.MatchString(pkg.PkgPath) {
			rs = append(rs, pkg)
		}
	}

	return rs
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
				links = true
				fmt.Printf("# import chain %q\n", pathstr(append(path, pkg, imp)))
			}
			visit(imp, append(path, pkg))
		}
	}

	for _, pkg := range source {
		visit(pkg, nil)
	}

	return links
}

func flatten(pkgs []*packages.Package) []*packages.Package {
	var all []*packages.Package
	visited := map[string]bool{}
	var visit func(pkg *packages.Package)
	visit = func(pkg *packages.Package) {
		if _, visited := visited[pkg.PkgPath]; visited {
			return
		}
		visited[pkg.PkgPath] = true
		all = append(all, pkg)

		for _, imp := range pkg.Imports {
			visit(imp)
		}
	}

	for _, pkg := range pkgs {
		visit(pkg)
	}

	sort.Slice(all, func(i, k int) bool {
		return all[i].PkgPath < all[k].PkgPath
	})

	return all
}

func pathstr(path []*packages.Package) string {
	ids := []string{}
	for _, pkg := range path {
		ids = append(ids, pkg.ID)
	}

	return strings.Join(ids, " > ")
}
