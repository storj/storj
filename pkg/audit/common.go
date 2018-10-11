// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"github.com/zeebo/errs"
)

// Error is the default audit errs class
var Error = errs.Class("audit error")
