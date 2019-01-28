// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"github.com/zeebo/errs"
)

var (
	// ErrSetup is returned when there's an error with setup
	ErrSetup = errs.Class("setup error")
)
