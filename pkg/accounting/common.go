// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("tally error")
	mon   = monkit.Package()
)

// Interval is the datatype used in the aggregate accounting db
type Interval int

const (
	d1  Interval = iota
	d7  Interval = iota
	d30 Interval = iota
)
