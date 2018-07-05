// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"github.com/zeebo/errs"
)

// Error is the default boltdb errs class
var Error = errs.Class("netstate error")
