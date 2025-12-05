// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	// Error is a standard error class for this package.
	Error = errs.Class("repair checker")
	mon   = monkit.Package()
)
