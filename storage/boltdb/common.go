// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"github.com/zeebo/errs"
)

// Error is the default boltdb errs class
var Error = errs.Class("boltdb error")
