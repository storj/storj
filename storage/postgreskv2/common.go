// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv2

import (
	"github.com/zeebo/errs"
)

// Error is the default postgreskv errs class
var Error = errs.Class("postgreskv2 error")
