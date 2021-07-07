// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdAccessCreate struct {
	ex ulext.External

	accessPermissions

	token      string
	passphrase string
	name       string
	save       bool
}

func newCmdAccessCreate(ex ulext.External) *cmdAccessCreate {
	return &cmdAccessCreate{ex: ex}
}

func (c *cmdAccessCreate) Setup(params clingy.Parameters) {
	c.token = params.Flag("token", "Setup token from satellite UI (prompted if unspecified)", "").(string)
	c.passphrase = params.Flag("passphrase", "Passphrase used for encryption (prompted if unspecified)", "").(string)
	c.name = params.Flag("name", "Name to save newly created access, if --save is true", "default").(string)
	c.save = params.Flag("save", "Save the access", true, clingy.Transform(strconv.ParseBool)).(bool)

	c.accessPermissions.Setup(params)
}

func (c *cmdAccessCreate) Execute(ctx clingy.Context) error {
	return nil
}
