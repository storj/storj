// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdAccessRestrict struct {
	ex ulext.External
	am accessMaker

	access string
}

func newCmdAccessRestrict(ex ulext.External) *cmdAccessRestrict {
	return &cmdAccessRestrict{ex: ex}
}

func (c *cmdAccessRestrict) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Which access to restrict", "").(string)

	params.Break()
	c.am.Setup(params, c.ex, false)
}

func (c *cmdAccessRestrict) Execute(ctx clingy.Context) error {
	access, err := c.ex.OpenAccess(c.access)
	if err != nil {
		return err
	}

	return c.am.Execute(ctx, access)
}
