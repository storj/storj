// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package flightrecorder

// Config holds configuration for the Flight Recorder (Box), including separate buffer configurations for each event type.
type Config struct {
	Enabled bool `help:"enable flight recorder" default:"false"`

	DBStackFrameCapacity int `help:"capacity of the circular buffer for database stack frame events." default:"1000"`
}
