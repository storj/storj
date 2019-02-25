// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pdbclient

import (
	"github.com/zeebo/errs"
)

// Error is the pdbclient error class
var Error = errs.Class("pointerdb client error")
