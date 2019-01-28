// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"github.com/zeebo/errs"
)

// Error is the default postgreskv errs class
var Error = errs.Class("postgreskv error")
