// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"

	"github.com/zeebo/errs"
)

var (
	logDisposition = flag.String("log.disp", "prod",
		"switch to 'dev' to get more output")

	// Error is a process error class
	Error = errs.Class("proc error")
)
