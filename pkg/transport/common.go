// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	mon = monkit.Package()
	//Error is the errs class of standard Transport Client errors
	Error = errs.Class("transport error")
)

const (
	// defaultTransportDialTimeout is the default time to wait for a connection to be established.
	defaultTransportDialTimeout = 20 * time.Second

	// defaultTransportRequestTimeout is the default time to wait for a response.
	defaultTransportRequestTimeout = 20 * time.Second
)
