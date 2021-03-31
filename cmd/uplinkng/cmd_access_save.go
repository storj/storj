// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/uplink"
)

type cmdAccessSave struct {
	access string
	name   string
	force  bool
	use    bool
}

func (c *cmdAccessSave) Setup(a clingy.Arguments, f clingy.Flags) {
	c.access = f.New("access", "Access to save (prompted if unspecified)", "").(string)
	c.name = f.New("name", "Name to save the access grant under", "default").(string)

	c.force = f.New("force", "Force overwrite an existing saved access grant", false,
		clingy.Short('f'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.use = f.New("use", "Set the saved access to be the one used by default", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)
}

func (c *cmdAccessSave) Execute(ctx clingy.Context) error {
	// TODO(jeff): need to distinguish errors from reading the file from
	// errors due to the file not existing. otherwise, there's no way
	// to save the first access.

	accessDefault, accesses, err := gf.GetAccessInfo()
	if err != nil {
		return err
	}

	if c.access == "" {
		return errs.New("TODO: implement prompting for the access")
	}
	if _, err := uplink.ParseAccess(c.access); err != nil {
		return err
	}
	if _, ok := accesses[c.name]; ok && !c.force {
		return errs.New("Access %q already exists. Overwrite by specifying --force", c.name)
	}

	accesses[c.name] = c.access
	if c.use {
		accessDefault = c.name
	}

	return gf.SaveAccessInfo(accessDefault, accesses)
}
