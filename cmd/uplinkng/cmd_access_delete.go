// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type cmdAccessDelete struct {
	name string
}

func (c *cmdAccessDelete) Setup(a clingy.Arguments, f clingy.Flags) {
	c.name = a.New("name", "Access to delete").(string)
}

func (c *cmdAccessDelete) Execute(ctx clingy.Context) error {
	accessDefault, accesses, err := gf.GetAccessInfo()
	if err != nil {
		return err
	}
	if c.name == accessDefault {
		return errs.New("cannot delete current access")
	}
	if _, ok := accesses[c.name]; !ok {
		return errs.New("unknown access: %q", c.name)
	}
	delete(accesses, c.name)
	return gf.SaveAccessInfo(accessDefault, accesses)
}
