// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
)

type cmdAccessRestrict struct {
	ex ulext.External
	am accessMaker

	access   string
	importAs string
	exportTo string
}

func newCmdAccessRestrict(ex ulext.External) *cmdAccessRestrict {
	return &cmdAccessRestrict{ex: ex}
}

func (c *cmdAccessRestrict) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to restrict", "").(string)
	c.importAs = params.Flag("import-as", "Import the access as this name", "").(string)
	c.exportTo = params.Flag("export-to", "Export the access to this file path", "").(string)

	params.Break()
	c.am.Setup(params, c.ex)
	params.Break()
	c.am.perms.Setup(params, true)
}

func (c *cmdAccessRestrict) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	access, err := c.ex.OpenAccess(c.access)
	if err != nil {
		return err
	}

	access, err = c.am.Execute(ctx, c.importAs, access)
	if err != nil {
		return err
	}

	if c.exportTo != "" {
		return c.ex.ExportAccess(ctx, access, c.exportTo)
	}

	serialized, err := access.Serialize()
	if err != nil {
		return errs.Wrap(err)
	}

	_, _ = fmt.Fprintln(clingy.Stdout(ctx), serialized)
	return nil
}
