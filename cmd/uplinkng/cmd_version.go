// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/private/version"
)

type cmdVersion struct {
	verbose bool
}

func newCmdVersion() *cmdVersion {
	return &cmdVersion{}
}

func (c *cmdVersion) Setup(params clingy.Parameters) {
	c.verbose = params.Flag(
		"verbose", "prints all dependency versions", false,
		clingy.Short('v'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
}

func (c *cmdVersion) Execute(ctx clingy.Context) error {
	if version.Build.Release {
		fmt.Fprintln(ctx, "Release build")
	} else {
		fmt.Fprintln(ctx, "Development build")
	}

	{
		tw := newTabbedWriter(ctx.Stdout())
		if !version.Build.Version.IsZero() {
			tw.WriteLine("Version:", version.Build.Version.String())
		}
		if !version.Build.Timestamp.IsZero() {
			tw.WriteLine("Build timestamp:", version.Build.Timestamp.Format(time.RFC822))
		}
		if version.Build.CommitHash != "" {
			tw.WriteLine("Git commit:", version.Build.CommitHash)
		}
		tw.Done()
	}

	fmt.Fprintln(ctx)

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
