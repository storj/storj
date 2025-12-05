// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
)

type cmdAccessRemove struct {
	ex ulext.External

	access string
}

func newCmdAccessRemove(ex ulext.External) *cmdAccessRemove {
	return &cmdAccessRemove{ex: ex}
}

func (c *cmdAccessRemove) Setup(params clingy.Parameters) {
	c.access = params.Arg("name", "Access name to delete").(string)
}

func (c *cmdAccessRemove) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	accessInfoFile, err := c.ex.AccessInfoFile()
	if err != nil {
		return errs.Wrap(err)
	}

	defaultName, accesses, err := c.ex.GetAccessInfo(true)
	if err != nil {
		return err
	}

	if c.access == defaultName {
		return errs.New("cannot delete current access")
	}
	if _, ok := accesses[c.access]; !ok {
		return errs.New("unknown access: %q", c.access)
	}

	delete(accesses, c.access)
	if err := c.ex.SaveAccessInfo(defaultName, accesses); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Removed access %q from %q\n", c.access, accessInfoFile)

	return nil
}
