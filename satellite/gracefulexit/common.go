// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	// Error is the default error class for graceful exit package.
	Error = errs.Class("gracefulexit")

	// ErrNodeNotFound is returned if a graceful exit entry for a  node does not exist in database.
	ErrNodeNotFound = errs.Class("graceful exit node not found")

	mon = monkit.Package()
)

// Config for the chore.
type Config struct {
	Enabled   bool `help:"whether or not graceful exit is enabled on the satellite side." default:"true"`
	TimeBased bool `help:"whether graceful exit will be determined by a period of time, rather than by instructing nodes to transfer one piece at a time" default:"true" hidden:"true"`

	NodeMinAgeInMonths int `help:"minimum age for a node on the network in order to initiate graceful exit" default:"6" testDefault:"0"`

	GracefulExitDurationInDays int           `help:"number of days it takes to execute a passive graceful exit" default:"30" testDefault:"1"`
	OfflineCheckInterval       time.Duration `help:"how frequently to check uptime ratio of gracefully-exiting nodes" default:"30m" testDefault:"10s"`
	MinimumOnlineScore         float64       `help:"a gracefully exiting node will fail GE if it falls below this online score (compare AuditHistoryConfig.OfflineThreshold)" default:"0.8"`
}
