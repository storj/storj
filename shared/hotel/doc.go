// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package hotel provides helpers to use OpenTelemetry with a Monkit-like API.
//
// The goal is to provide drop-ing replacement of monkit API with Opentelemetry backend. Some utilities can be useful even in the future
// (for example the mon.Task is a useful helper), while some others can be replaced in the future (the current monkit counters are not aware of current span/ctx).
package hotel
