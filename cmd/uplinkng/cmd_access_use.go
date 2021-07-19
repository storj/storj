// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdAccessUse struct {
	ex ulext.External

	name string
}

func newCmdAccessUse(ex ulext.External) *cmdAccessUse {
	return &cmdAccessUse{ex: ex}
}

func (c *cmdAccessUse) Setup(params clingy.Parameters) {
	c.name = params.Arg("name", "Access to use").(string)
}

func (c *cmdAccessUse) Execute(ctx clingy.Context) error {
	_, accesses, err := c.ex.GetAccessInfo(true)
	if err != nil {
		return err
	}
	if _, ok := accesses[c.name]; !ok {
		return errs.New("unknown access: %q", c.name)
	}
	return c.ex.SaveAccessInfo(c.name, accesses)
}
