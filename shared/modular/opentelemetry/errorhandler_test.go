// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
)

func TestErrorHandlerSuppressesRepeats(t *testing.T) {
	var out strings.Builder
	handler := newErrorHandler(&out, time.Minute)

	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	handler.now = func() time.Time { return now }

	for i := 0; i < 5; i++ {
		handler.Handle(errs.New("connection refused"))
		now = now.Add(time.Second)
	}

	lines := nonEmptyLines(out.String())
	require.Len(t, lines, 1)
	require.Equal(t, "10:00:00.000\tERROR\totel\tconnection refused", lines[0])

	// a different message is not suppressed.
	handler.Handle(errs.New("other problem"))
	// after the interval, the original message is printed again, with the count.
	now = now.Add(time.Minute)
	handler.Handle(errs.New("connection refused"))

	lines = nonEmptyLines(out.String())
	require.Len(t, lines, 3)
	require.Contains(t, lines[1], "other problem")
	require.Contains(t, lines[2], "connection refused")
}

func TestErrorHandlerCountsSuppressed(t *testing.T) {
	var out strings.Builder
	handler := newErrorHandler(&out, time.Minute)

	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	handler.now = func() time.Time { return now }

	handler.Handle(errs.New("boom"))
	for i := 0; i < 3; i++ {
		handler.Handle(errs.New("boom"))
	}
	now = now.Add(2 * time.Minute)
	handler.Handle(errs.New("boom"))

	lines := nonEmptyLines(out.String())
	require.Len(t, lines, 2)
	require.Contains(t, lines[1], "(3 identical errors suppressed)")
}

// TestUnreachableDestination verifies that an OTLP destination nobody is
// listening on does not prevent startup, and does not stop records from
// reaching the other configured exporters.
func TestUnreachableDestination(t *testing.T) {
	ctx := context.Background()

	otel, err := NewOpentelemetry(ctx, Config{
		Service: "storj",
		Logging: Logging{
			// port 1 is reserved and never has a listener.
			HTTPDestination: "localhost:1",
			Stdout:          "none",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, otel.Log)

	var record otellog.Record
	record.SetBody(otellog.StringValue("hello"))
	otel.Log.Logger("test").Emit(ctx, record)

	require.NoError(t, otel.Log.ForceFlush(ctx))
	require.NoError(t, otel.Log.Shutdown(ctx))
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

var _ log.Exporter = (*prettyExporter)(nil)
