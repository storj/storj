// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package quic

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	mon = monkit.Package()

	// Error is a pkg/quic error.
	Error = errs.Class("quic")
)
