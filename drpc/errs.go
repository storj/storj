// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpc

import "github.com/zeebo/errs"

var (
	Error         = errs.Class("drpc")
	InternalError = errs.Class("internal error")
	ProtocolError = errs.Class("protocol error")
)
