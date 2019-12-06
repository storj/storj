// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("rollup error")
	mon   = monkit.Package()
)
