// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"github.com/zeebo/errs"
)

// Error is the errs class of standard segment errors
var Error = errs.Class("segment error")

// IrreparableError is the errs class of irreparable segment errors
var IrreparableError = errs.Class("irreparable error")
