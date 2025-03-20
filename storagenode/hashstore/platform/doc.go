// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// package platform provides platform specific code for the hashstore.
package platform

import "github.com/zeebo/errs"

// Error wraps errors returned by this package.
var Error = errs.Class("platform")
