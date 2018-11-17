// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package irrdbclient

import (
	"github.com/zeebo/errs"
)

// Error is the irreparabledb error class
var Error = errs.Class("irreparabledb client error")
