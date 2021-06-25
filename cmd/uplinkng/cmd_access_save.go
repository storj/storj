// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/uplink"
)

type cmdAccessSave struct {
	ex ulext.External
	am accessMaker

	access string
}

func newCmdAccessSave(ex ulext.External) *cmdAccessSave {
	return &cmdAccessSave{ex: ex}
}

func (c *cmdAccessSave) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Serialized access value to save (prompted if unspecified)", "").(string)

	params.Break()
	c.am.Setup(params, c.ex, amSaveForced)
}

func (c *cmdAccessSave) Execute(ctx clingy.Context) (err error) {
	if c.access == "" {
		c.access, err = c.ex.PromptInput(ctx, "Access:")
		if err != nil {
			return errs.Wrap(err)
		}
	}

	access, err := uplink.ParseAccess(c.access)
	if err != nil {
		return err
	}

	return c.am.Execute(ctx, access)
}
