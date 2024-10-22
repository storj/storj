// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"github.com/zeebo/errs"
)

// Constants for accounting_raw, accounting_rollup, and accounting_timestamps.
const (
	// LastAtRestTally represents the accounting timestamp for the at-rest data calculation.
	LastAtRestTally = "LastAtRestTally"
	// LastBandwidthTally represents the accounting timestamp for the bandwidth allocation query.
	LastBandwidthTally = "LastBandwidthTally"
	// LastRollup represents the accounting timestamp for rollup calculations.
	LastRollup = "LastRollup"
)

var (
	// ErrInvalidArgument is returned when a function argument has an invalid
	// business domain value.
	ErrInvalidArgument = errs.Class("invalid argument")
	// ErrSystemOrNetError is returned when the used storage backend returns an
	// internal system or network error.
	ErrSystemOrNetError = errs.Class("accounting backend")
	// ErrKeyNotFound is returned when the key is not found in the cache.
	ErrKeyNotFound = errs.Class("key not found")
	// ErrUnexpectedValue is returned when an unexpected value according the
	// business domain is in the cache.
	ErrUnexpectedValue = errs.Class("unexpected value")
)
