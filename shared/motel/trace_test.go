// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package motel

import (
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestProvider(t *testing.T) {
	tp := &TraceProvider{}

	otel.SetTracerProvider(tp)
	otel.GetTracerProvider()
	trace.NewTracerProvider()

}
