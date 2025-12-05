// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
)

type cmdAccessImport struct {
	ex ulext.External
	am accessMaker

	name   string
	access string
}

func newCmdAccessImport(ex ulext.External) *cmdAccessImport {
	return &cmdAccessImport{ex: ex}
}

func (c *cmdAccessImport) Setup(params clingy.Parameters) {
	c.am.Setup(params, c.ex)

	c.name = params.Arg("name", "Name to save the access as").(string)
	c.access = params.Arg("access|filename", "Serialized access value or file path to save").(string)
}

func (c *cmdAccessImport) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if c.name == "" {
		return errs.New("Must specify a name to import the access as.")
	}

	access, err := parseAccessDataOrPossiblyFile(c.access)
	if err != nil {
		return errs.Wrap(err)
	}

	_, err = c.am.Execute(ctx, c.name, access)
	return err
}
