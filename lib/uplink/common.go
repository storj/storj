// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"github.com/zeebo/errs"
	"github.com/zeebo/goof"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	Troop goof.Troop
	mon   = monkit.Package()

	// Error is the toplevel class of errors for the uplink library.
	Error = errs.Class("libuplink")
)
