// Copyright (C) 2020 Storj Labs, Incache.
// See LICENSE for copying information.

// Package nodeselection implements node selection logic.
package nodeselection

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	mon = monkit.Package()
	// Error represents an nodeselection error.
	Error = errs.Class("nodeselection")
)
