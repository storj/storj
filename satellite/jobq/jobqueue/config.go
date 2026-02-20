// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package jobqueue

import (
	"time"

	"storj.io/common/memory"
)

// Config holds the configuration for the job queue server-side queue.
type Config struct {
	// InitAlloc is the initial allocation size for the job queue, in bytes.
	// There is no special need to keep this low; unused parts of the queue
	// allocation will not take up system memory until the queue grows to that
	// size.
	InitAlloc memory.Size `help:"initial allocation size for the job queue, in bytes" default:"2GiB"`
	// MaxMemPerPlacement is the maximum memory to be used per placement for
	// storing jobs ready for repair, in bytes. The queue will not actually
	// consume this amount of memory unless it is full. If full, lower-priority
	// or longer-delayed jobs will be evicted from the queue when new jobs are
	// added.
	MaxMemPerPlacement memory.Size `help:"maximum memory per placement, in bytes" default:"4GiB"`
	// MemReleaseThreshold is the memory release threshold for the job queue, in
	// bytes. When the job queue has more than this amount of memory mapped to
	// empty pages (because the queue shrunk considerably), the unused memory
	// will be marked as unused (if supported) and the OS will be allowed to
	// reclaim it.
	MemReleaseThreshold memory.Size `help:"element memory release threshold for the job queue, in bytes" default:"100MiB"`
	// RetryAfter is the time to wait before retrying a failed job. If jobs are
	// pushed to the queue with a LastAttemptedAt more recent than this duration
	// ago, they will go into the retry queue instead of the repair queue, until
	// they are eligible to go in the repair queue.
	RetryAfter time.Duration `help:"time to wait before retrying a failed job" default:"1h"`
}
