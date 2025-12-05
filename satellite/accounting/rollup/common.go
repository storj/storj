// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rollup

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("rollup")
	mon   = monkit.Package()
)
