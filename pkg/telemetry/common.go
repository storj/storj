// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"github.com/zeebo/errs"
)

// Error is the default telemetry errs class
var Error = errs.Class("telemetry error")
