// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"io/ioutil"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/uplink"
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

func (c *cmdAccessImport) Execute(ctx clingy.Context) (err error) {
	if c.name == "" {
		return errs.New("Must specify a name to import the access as.")
	}

	access, err := uplink.ParseAccess(c.access)
	if err != nil {
		data, err := ioutil.ReadFile(c.access)
		if err != nil {
			return errs.Wrap(err)
		}
		access, err = uplink.ParseAccess(string(bytes.TrimSpace(data)))
		if err != nil {
			return errs.Wrap(err)
		}
	}

	_, err = c.am.Execute(ctx, c.name, access)
	return err
}
