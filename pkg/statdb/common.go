// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	// Error is the default boltdb errs class
	Error = errs.Class("statdb error")
	mon   = monkit.Package()
)
