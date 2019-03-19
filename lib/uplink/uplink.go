// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"github.com/zeebo/errs"
)

var (
	// Error is the errs class of standard End User Client errors
	Error = errs.Class("libuplink error")
)
