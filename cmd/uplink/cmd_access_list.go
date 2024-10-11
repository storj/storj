// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/uplink"
)

type cmdAccessList struct {
	ex ulext.External

	verbose bool
}

func newCmdAccessList(ex ulext.External) *cmdAccessList {
	return &cmdAccessList{ex: ex}
}

func (c *cmdAccessList) Setup(params clingy.Parameters) {
	c.verbose = params.Flag("verbose", "Verbose output of accesses", false,
		clingy.Short('v'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
}

func (c *cmdAccessList) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	defaultName, accesses, err := c.ex.GetAccessInfo(true)
	if err != nil {
		return err
	}

	var tw *tabbedWriter
	if c.verbose {
		tw = newTabbedWriter(clingy.Stdout(ctx), "CURRENT", "NAME", "SATELLITE", "VALUE")
	} else {
		tw = newTabbedWriter(clingy.Stdout(ctx), "CURRENT", "NAME", "SATELLITE")
	}
	defer tw.Done()

	var names []string
	for name := range accesses {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		address := "<access parse error>"

		if access, err := uplink.ParseAccess(accesses[name]); err == nil {
			address = access.SatelliteAddress()
			if idx := strings.IndexByte(address, '@'); !c.verbose && idx >= 0 {
				address = address[idx+1:]
			}
		}

		inUse := ' '
		if name == defaultName {
			inUse = '*'
		}

		if c.verbose {
			tw.WriteLine(inUse, name, address, accesses[name])
		} else {
			tw.WriteLine(inUse, name, address)
		}
	}

	return nil
}
