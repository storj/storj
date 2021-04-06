// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type cmdVersion struct {
	verbose bool
}

func (c *cmdVersion) Setup(a clingy.Arguments, f clingy.Flags) {
	c.verbose = f.New(
		"verbose", "prints all dependency versions", false,
		clingy.Short('v'),
		clingy.Transform(strconv.ParseBool)).(bool)
}

func (c *cmdVersion) Execute(ctx clingy.Context) error {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return errs.New("unable to read build info")
	}

	tw := newTabbedWriter(ctx.Stdout(), "PATH", "VERSION")
	defer tw.Done()

	tw.WriteLine(bi.Main.Path, bi.Main.Version)
	for _, mod := range bi.Deps {
		if c.verbose || strings.HasPrefix(mod.Path, "storj.io/") {
			tw.WriteLine(mod.Path, mod.Version)
		}
	}

	return nil
}
