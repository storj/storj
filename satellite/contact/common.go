// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/eventkit"
)

var (
	// Error is the default error class for contact package.
	Error = errs.Class("contact")

	mon = monkit.Package()

	ek = eventkit.Package()
)
