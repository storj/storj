// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"github.com/zeebo/errs"
)

// Error is the default auth error class
var Error = errs.Class("auth error")
