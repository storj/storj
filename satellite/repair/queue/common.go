// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package queue

import (
	"github.com/zeebo/errs"
)

// Error is a standard error class for this package.
var Error = errs.Class("repair queue")

// ErrEmpty is returned when attempting to Dequeue from an empty queue.
var ErrEmpty = errs.Class("empty queue")
