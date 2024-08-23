// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import "github.com/zeebo/errs"

var (
	// ErrInvalidTaxID is returned when a tax ID value is invalid.
	ErrInvalidTaxID = errs.Class("Invalid tax ID value")
	// ErrUnbilledUsage is error type of unbilled usage.
	ErrUnbilledUsage = errs.Class("Unbilled usage")
	// ErrUnbilledUsageCurrentMonth occurs when a project has unbilled usage for the current month.
	ErrUnbilledUsageCurrentMonth = ErrUnbilledUsage.New("usage for current month exists")
	// ErrUnbilledUsageLastMonth occurs when a project has unbilled usage for the previous month.
	ErrUnbilledUsageLastMonth = ErrUnbilledUsage.New("usage for last month exists, but is not billed yet")
)
