// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package sdbclient

import (
	"github.com/zeebo/errs"
)

// Error is the sdbclient error class
var Error = errs.Class("statdb client error")
