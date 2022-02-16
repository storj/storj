// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

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

func (c *cmdAccessUse) Execute(ctx clingy.Context) error {
	_, accesses, err := c.ex.GetAccessInfo(true)
	if err != nil {
		return err
	}
	if _, ok := accesses[c.access]; !ok {
		return errs.New("unknown access: %q", c.access)
	}
	if err := c.ex.SaveAccessInfo(c.access, accesses); err != nil {
		return err
	}

	fmt.Fprintf(ctx, "Switched default access to %q\n", c.access)

	return nil
}
