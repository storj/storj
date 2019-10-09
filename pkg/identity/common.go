// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity // import "storj.io/storj/pkg/identity"

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	mon = monkit.Package()

	// Error is a pkg/identity error
	Error = errs.Class("identity error")
)
