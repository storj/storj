// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulext"
)

type cmdAccessUse struct {
	ex ulext.External

	access string
}

func newCmdAccessUse(ex ulext.External) *cmdAccessUse {
	return &cmdAccessUse{ex: ex}
}

func (c *cmdAccessUse) Setup(params clingy.Parameters) {
	c.access = params.Arg("access", "Access name to use").(string)
}

func (c *cmdAccessUse) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, accesses, err := c.ex.GetAccessInfo(true)
	if err != nil {
		return err
	}
	if _, ok := accesses[c.access]; !ok {
		return fmt.Errorf("ERROR: access %q does not exist. Use 'uplink access list' to see existing accesses", c.access)
	}
	if err := c.ex.SaveAccessInfo(c.access, accesses); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Switched default access to %q\n", c.access)

	return nil
}
