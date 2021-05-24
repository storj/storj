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

func (c *cmdAccessCreate) Setup(a clingy.Arguments, f clingy.Flags) {
	c.token = f.New("token", "Setup token from satellite UI (prompted if unspecified)", "").(string)
	c.passphrase = f.New("passphrase", "Passphrase used for encryption (prompted if unspecified)", "").(string)
	c.name = f.New("name", "Name to save newly created access, if --save is true", "default").(string)
	c.save = f.New("save", "Save the access", true, clingy.Transform(strconv.ParseBool)).(bool)

	c.accessPermissions.Setup(a, f)
}

func (c *cmdAccessCreate) Execute(ctx clingy.Context) error {
	return nil
}
