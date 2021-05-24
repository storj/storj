// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type cmdAccessUse struct {
	name string
}

func (c *cmdAccessUse) Setup(a clingy.Arguments, f clingy.Flags) {
	c.name = a.New("name", "Access to use").(string)
}

func (c *cmdAccessUse) Execute(ctx clingy.Context) error {
	_, accesses, err := gf.GetAccessInfo()
	if err != nil {
		return err
	}
	if _, ok := accesses[c.name]; !ok {
		return errs.New("unknown access: %q", c.name)
	}
	return gf.SaveAccessInfo(c.name, accesses)
}
