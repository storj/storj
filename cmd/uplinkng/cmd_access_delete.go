// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdAccessDelete struct {
	ex ulext.External

	name string
}

func newCmdAccessDelete(ex ulext.External) *cmdAccessDelete {
	return &cmdAccessDelete{ex: ex}
}

func (c *cmdAccessDelete) Setup(params clingy.Parameters) {
	c.name = params.Arg("name", "Access to delete").(string)
}

func (c *cmdAccessDelete) Execute(ctx clingy.Context) error {
	defaultName, accesses, err := c.ex.GetAccessInfo(true)
	if err != nil {
		return err
	}
	if c.name == defaultName {
		return errs.New("cannot delete current access")
	}
	if _, ok := accesses[c.name]; !ok {
		return errs.New("unknown access: %q", c.name)
	}
	delete(accesses, c.name)
	return c.ex.SaveAccessInfo(defaultName, accesses)
}
