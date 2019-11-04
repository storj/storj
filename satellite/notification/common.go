// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	// Error is the default error class for notification package.
	Error = errs.Class("notification")

	mon = monkit.Package()
)

// Config for the Notification System
type Config struct {
	HourlyEmails int `help:"maximum amount of emails per node" default:"5"`
	HourlyRPC    int `help:"maximum amount of rpc messages per node" default:"360"`
}
