// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"
)

type cmdCp struct {
	projectProvider

	recursive bool
	source    string
	dest      string
}

func (c *cmdCp) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)

	c.recursive = f.New("recursive", "Peform a recursive copy", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)

	c.source = a.New("source", "Source to copy").(string)
	c.dest = a.New("dest", "Desination to copy").(string)
}

func (c *cmdCp) Execute(ctx clingy.Context) error {
	return nil
}
