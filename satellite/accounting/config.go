// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

// Config is the configuration struct for the accounting package.
type Config struct {
	RetentionRemainderRecorder RetentionRemainderRecorderConfig `help:"configuration for the retention remainder recorder"`
}
