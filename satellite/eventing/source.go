// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"context"
	"time"

	"storj.io/storj/satellite/metabase"
)

// ChangeEvent is a backend-neutral representation of a single object mutation
// that may generate an S3 bucket notification. It carries the decoded fields
// the service needs without any Spanner- or TiDB-specific types.
type ChangeEvent struct {
	EventName string
	metabase.ObjectStream
	TotalPlainSize  int64
	CommitTimestamp time.Time
}

// EventSource abstracts over the backend-specific record delivery loop.
// Implementations decode backend records into ChangeEvents and call fn for
// each one. Listen blocks until ctx is cancelled or a permanent error occurs.
type EventSource interface {
	Listen(ctx context.Context, fn func(event ChangeEvent) (PendingResult, error)) error
}
