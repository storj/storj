// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdAccessCreate struct {
	ex ulext.External
	am accessMaker

	token      string
	passphrase string
}

func newCmdAccessCreate(ex ulext.External) *cmdAccessCreate {
	return &cmdAccessCreate{ex: ex}
}

func (c *cmdAccessCreate) Setup(params clingy.Parameters) {
	c.token = params.Flag("token", "Setup token from satellite UI (prompted if unspecified)", "").(string)
	c.passphrase = params.Flag("passphrase", "Passphrase used for encryption (prompted if unspecified)", "").(string)

	params.Break()
	c.am.Setup(params, c.ex, amSaveDefaultTrue)
}

func (c *cmdAccessCreate) Execute(ctx clingy.Context) (err error) {
	if c.token == "" {
		c.token, err = c.ex.PromptInput(ctx, "Setup token:")
		if err != nil {
			return errs.Wrap(err)
		}
	}

	if c.passphrase == "" {
		// TODO: secret prompt
		c.passphrase, err = c.ex.PromptInput(ctx, "Passphrase:")
		if err != nil {
			return errs.Wrap(err)
		}
	}

	access, err := c.ex.RequestAccess(ctx, c.token, c.passphrase)
	if err != nil {
		return errs.Wrap(err)
	}

	return c.am.Execute(ctx, access)
}
