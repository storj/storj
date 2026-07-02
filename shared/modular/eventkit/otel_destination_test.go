// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventkit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"storj.io/eventkit"
)

// memoryExporter collects exported log records in memory for assertions.
type memoryExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memoryExporter) Export(ctx context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

func (e *memoryExporter) Shutdown(ctx context.Context) error   { return nil }
func (e *memoryExporter) ForceFlush(ctx context.Context) error { return nil }

func (e *memoryExporter) collected() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]sdklog.Record, len(e.records))
	copy(out, e.records)
	return out
}

func TestOtelDestination(t *testing.T) {
	ctx := context.Background()
	exporter := &memoryExporter{}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)),
	)
	defer func() { require.NoError(t, provider.Shutdown(ctx)) }()

	dest := newOtelDestination(provider)

	ts := time.Unix(1700000000, 500).UTC()
	dest.Submit(&eventkit.Event{
		Name:      "upload",
		Scope:     []string{"storagenode", "pieces"},
		Timestamp: ts,
		Tags: []eventkit.Tag{
			eventkit.String("piece_id", "abc"),
			eventkit.Int64("size", 1024),
			eventkit.Float64("ratio", 0.5),
			eventkit.Bool("success", true),
			eventkit.Bytes("blob", []byte{1, 2, 3}),
			eventkit.Duration("elapsed", 2*time.Second),
		},
	})

	records := exporter.collected()
	require.Len(t, records, 1)

	rec := records[0]
	require.Equal(t, "upload", rec.EventName())
	require.Equal(t, "upload", rec.Body().AsString())
	require.Equal(t, ts, rec.Timestamp())
	require.Equal(t, otellog.SeverityInfo, rec.Severity())

	attrs := map[string]otellog.Value{}
	rec.WalkAttributes(func(kv otellog.KeyValue) bool {
		attrs[kv.Key] = kv.Value
		return true
	})

	require.Equal(t, "upload", attrs["name"].AsString())
	require.Equal(t, "storagenode.pieces", attrs["scope"].AsString())
	require.Equal(t, "abc", attrs["piece_id"].AsString())
	require.Equal(t, int64(1024), attrs["size"].AsInt64())
	require.Equal(t, 0.5, attrs["ratio"].AsFloat64())
	require.Equal(t, true, attrs["success"].AsBool())
	require.Equal(t, []byte{1, 2, 3}, attrs["blob"].AsBytes())
	require.Equal(t, int64(2*time.Second), attrs["elapsed"].AsInt64())
}
