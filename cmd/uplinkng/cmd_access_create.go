// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"
)

type cmdAccessCreate struct {
	accessPermissions

	token      string
	passphrase string
	name       string
	save       bool
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
