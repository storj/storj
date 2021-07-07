// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/uplink"
)

type cmdAccessSave struct {
	ex ulext.External

	access string
	name   string
	force  bool
	use    bool
}

func newCmdAccessSave(ex ulext.External) *cmdAccessSave {
	return &cmdAccessSave{ex: ex}
}

func (c *cmdAccessSave) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access to save (prompted if unspecified)", "").(string)
	c.name = params.Flag("name", "Name to save the access grant under", "default").(string)

	c.force = params.Flag("force", "Force overwrite an existing saved access grant", false,
		clingy.Short('f'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.use = params.Flag("use", "Set the saved access to be the one used by default", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)
}

func (c *cmdAccessSave) Execute(ctx clingy.Context) error {
	defaultName, accesses, err := c.ex.GetAccessInfo(false)
	if err != nil {
		return err
	}

	if c.access == "" {
		c.access, err = c.ex.PromptInput(ctx, "Access:")
		if err != nil {
			return errs.Wrap(err)
		}
	}

	if _, err := uplink.ParseAccess(c.access); err != nil {
		return err
	}
	if _, ok := accesses[c.name]; ok && !c.force {
		return errs.New("Access %q already exists. Overwrite by specifying --force or choose a new name with --name", c.name)
	}

	accesses[c.name] = c.access
	if c.use || defaultName == "" {
		defaultName = c.name
	}

	return c.ex.SaveAccessInfo(defaultName, accesses)
}
