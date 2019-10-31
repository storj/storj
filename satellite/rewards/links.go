// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"github.com/zeebo/errs"
)

var (
	// NoMatchPartnerIDErr is the error class used when an offer has reached its redemption capacity
	NoMatchPartnerIDErr = errs.Class("partner not exist")
)
