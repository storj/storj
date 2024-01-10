// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package uploadselection implements node selection logic for uploads.
package nodeselection

import (
	"github.com/zeebo/errs"
)

var (
	// Error represents an uploadselection error.
	Error = errs.Class("uploadselection")
)
