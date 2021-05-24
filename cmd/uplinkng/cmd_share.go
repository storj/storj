// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import "github.com/zeebo/clingy"

type cmdShare struct {
	projectProvider
}

func (c *cmdShare) Setup(a clingy.Arguments, f clingy.Flags) {
	c.projectProvider.Setup(a, f)
}

func (c *cmdShare) Execute(ctx clingy.Context) error {
	return nil
}
