// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdAccessRevoke struct {
	ex ulext.External
}

func newCmdAccessRevoke(ex ulext.External) *cmdAccessRevoke {
	return &cmdAccessRevoke{ex: ex}
}

func (c *cmdAccessRevoke) Setup(params clingy.Parameters) {
}

func (c *cmdAccessRevoke) Execute(ctx clingy.Context) error {
	return nil
}
