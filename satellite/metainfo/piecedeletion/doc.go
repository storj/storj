// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package piecedeletion implements service for deleting pieces that combines concurrent requests.
package piecedeletion

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var mon = monkit.Package()

// Error is the default error class for piece deletion.
var Error = errs.Class("piece deletion")
