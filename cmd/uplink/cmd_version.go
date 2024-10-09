// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/version"
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

func (c *cmdVersion) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if version.Build.Release {
		_, _ = fmt.Fprintln(clingy.Stdout(ctx), "Release build")
	} else {
		_, _ = fmt.Fprintln(clingy.Stdout(ctx), "Development build")
	}

	{
		tw := newTabbedWriter(clingy.Stdout(ctx))
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

	_, _ = fmt.Fprintln(clingy.Stdout(ctx))

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return errs.New("unable to read build info")
	}

	tw := newTabbedWriter(clingy.Stdout(ctx), "PATH", "VERSION")
	defer tw.Done()

	tw.WriteLine(bi.Main.Path, bi.Main.Version)
	for _, mod := range bi.Deps {
		if c.verbose || strings.HasPrefix(mod.Path, "storj.io/") {
			tw.WriteLine(mod.Path, mod.Version)
		}
	}

	return nil
}
