// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package boltdb

import (
	"github.com/zeebo/errs"
)

// Error is the default boltdb errs class
var Error = errs.Class("boltdb error")

// ErrKeyNotFound should occur when a key isn't found in a boltdb bucket (table)
var ErrKeyNotFound = errs.Class("key not found")
