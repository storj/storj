// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package motel provides types and functions for using Opentelemetry with a Monkit backend.
//
// Backends of Opentelemetry can be configured. This package makes it possible to use the Opentelemetry API, but using Monkit under the hood.
// Monkit will manage all the traces/spans, metrics and the exporting of the data.
package motel
