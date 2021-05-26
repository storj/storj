// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/zeebo/clingy"

	"storj.io/uplink"
)

type cmdAccessList struct {
	verbose bool
}

func (c *cmdAccessList) Setup(a clingy.Arguments, f clingy.Flags) {
	c.verbose = f.New("verbose", "Verbose output of accesses", false,
		clingy.Short('v'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
}

func (c *cmdAccessList) Execute(ctx clingy.Context) error {
	accessDefault, accesses, err := gf.GetAccessInfo()
	if err != nil {
		return err
	}

	var tw *tabbedWriter
	if c.verbose {
		tw = newTabbedWriter(ctx.Stdout(), "CURRENT", "NAME", "SATELLITE", "VALUE")
	} else {
		tw = newTabbedWriter(ctx.Stdout(), "CURRENT", "NAME", "SATELLITE")
	}
	defer tw.Done()

	var names []string
	for name := range accesses {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		access, err := uplink.ParseAccess(accesses[name])
		if err != nil {
			return err
		}
		address := access.SatelliteAddress()
		if idx := strings.IndexByte(address, '@'); !c.verbose && idx >= 0 {
			address = address[idx+1:]
		}

		inUse := ' '
		if name == accessDefault {
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
