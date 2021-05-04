// Copyright (C) 2020 Storj Labs, Incache.
// See LICENSE for copying information.

// Package uploadselection implements node selection logic for uploads.
package uploadselection

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	mon = monkit.Package()
	// Error represents an uploadselection error.
	Error = errs.Class("uploadselection")
)
