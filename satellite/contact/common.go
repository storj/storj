// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

// Error is the default error class for contact package.
var Error = errs.Class("contact")

var mon = monkit.Package()
