// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"github.com/zeebo/errs"
)

// Error is the default audit errs class
var Error = errs.Class("audit error")

// ContainError is the containment errs class
var ContainError = errs.Class("containment error")

// ErrContainedNotFound is the errs class for when a pending audit isn't found
var ErrContainedNotFound = errs.Class("pending audit not found")
