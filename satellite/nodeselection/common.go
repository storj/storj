// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package uploadselection implements node selection logic for uploads.
package nodeselection

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	mon = monkit.Package()

	// Error represents an uploadselection error.
	Error = errs.Class("uploadselection")
)
