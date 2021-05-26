// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdShare struct {
	ex ulext.External

	access string
}

func newCmdShare(ex ulext.External) *cmdShare {
	return &cmdShare{ex: ex}
}

func (c *cmdShare) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Which access to use", "").(string)
}

func (c *cmdShare) Execute(ctx clingy.Context) error {
	return nil
}
