// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
)

func TestPrettyExporter(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	provider := log.NewLoggerProvider(
		log.WithProcessor(log.NewSimpleProcessor(newPrettyExporter(&buf))),
	)
	defer func() { require.NoError(t, provider.Shutdown(ctx)) }()

	logger := provider.Logger("myscope")
	var rec otellog.Record
	rec.SetBody(otellog.StringValue("hello world"))
	rec.SetSeverity(otellog.SeverityInfo)
	rec.AddAttributes(
		otellog.String("piece_id", "abc"),
		otellog.Int64("size", 1024),
	)
	logger.Emit(ctx, rec)

	out := buf.String()
	require.Contains(t, out, "myscope")
	require.Contains(t, out, "hello world")
	require.Contains(t, out, "piece_id=abc")
	require.Contains(t, out, "size=1024")
	// readable, single line (plus trailing newline), not JSON.
	require.False(t, strings.HasPrefix(strings.TrimSpace(out), "{"))
	require.Equal(t, 1, strings.Count(out, "\n"))
}

func TestFilterEventkit(t *testing.T) {
	emit := func(printEventkit bool, scope string) string {
		ctx := context.Background()
		var buf bytes.Buffer
		provider := log.NewLoggerProvider(
			log.WithProcessor(filterEventkit(log.NewSimpleProcessor(newPrettyExporter(&buf)), printEventkit)),
		)
		defer func() { _ = provider.Shutdown(ctx) }()

		var rec otellog.Record
		rec.SetBody(otellog.StringValue("body"))
		provider.Logger(scope).Emit(ctx, rec)
		return buf.String()
	}

	// eventkit scopes are dropped by default.
	require.Empty(t, emit(false, "eventkit"))
	require.Empty(t, emit(false, "eventkit:eventkit"))

	// non-eventkit scopes always pass.
	require.NotEmpty(t, emit(false, "storj"))
	require.NotEmpty(t, emit(false, "storagenode:pieces"))

	// with print-eventkit enabled, eventkit scopes pass too.
	require.NotEmpty(t, emit(true, "eventkit"))
	require.NotEmpty(t, emit(true, "eventkit:eventkit"))
}
