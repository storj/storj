// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"time"

	"storj.io/storj/satellite/metabase/changestream"
)

// PendingResult is an alias for changestream.PendingResult.
// It is defined here so that future backends (e.g. TiDBEventSource) can
// reference the type without importing the Spanner-specific changestream package.
type PendingResult = changestream.PendingResult

// ImmediateResult returns a PendingResult that is already resolved with the
// given timestamp. Delegates to changestream.ImmediateResult.
func ImmediateResult(timestamp time.Time) PendingResult {
	return changestream.ImmediateResult(timestamp)
}
