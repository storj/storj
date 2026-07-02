// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
)

// prettyExporter is a log.Exporter that writes human-readable, single-line log
// records to a writer (typically stdout), instead of the JSON output produced by
// the standard stdoutlog exporter.
type prettyExporter struct {
	mu sync.Mutex
	w  io.Writer
}

// newPrettyExporter creates a prettyExporter writing to w.
func newPrettyExporter(w io.Writer) *prettyExporter {
	return &prettyExporter{w: w}
}

// Export writes each record in a readable format.
func (e *prettyExporter) Export(ctx context.Context, records []log.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var b strings.Builder
	for i := range records {
		record := records[i]

		severity := record.SeverityText()
		if severity == "" {
			severity = record.Severity().String()
		}

		b.Reset()
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s",
			record.Timestamp().Format("15:04:05.000"),
			severity,
			record.InstrumentationScope().Name,
			record.Body().String(),
		)
		record.WalkAttributes(func(kv otellog.KeyValue) bool {
			fmt.Fprintf(&b, "\t%s=%s", kv.Key, kv.Value.String())
			return true
		})
		b.WriteByte('\n')

		if _, err := io.WriteString(e.w, b.String()); err != nil {
			return err
		}
	}
	return nil
}

// Shutdown is a no-op: the writer is not owned by the exporter.
func (e *prettyExporter) Shutdown(ctx context.Context) error { return nil }

// ForceFlush is a no-op: records are written synchronously on Export.
func (e *prettyExporter) ForceFlush(ctx context.Context) error { return nil }

// scopeFilterProcessor wraps another processor and only forwards records whose
// instrumentation scope name passes the allow predicate.
type scopeFilterProcessor struct {
	next  log.Processor
	allow func(scope string) bool
}

// OnEmit forwards the record to the wrapped processor unless it is filtered out.
func (p scopeFilterProcessor) OnEmit(ctx context.Context, record *log.Record) error {
	if !p.allow(record.InstrumentationScope().Name) {
		return nil
	}
	return p.next.OnEmit(ctx, record)
}

// Enabled delegates to the wrapped processor.
func (p scopeFilterProcessor) Enabled(ctx context.Context, param log.EnabledParameters) bool {
	return p.next.Enabled(ctx, param)
}

// Shutdown delegates to the wrapped processor.
func (p scopeFilterProcessor) Shutdown(ctx context.Context) error { return p.next.Shutdown(ctx) }

// ForceFlush delegates to the wrapped processor.
func (p scopeFilterProcessor) ForceFlush(ctx context.Context) error { return p.next.ForceFlush(ctx) }

// isEventkitScope reports whether an instrumentation scope belongs to eventkit.
// It matches both the eventkit event destination ("eventkit") and the eventkit
// component's own zap logs ("eventkit:eventkit").
func isEventkitScope(scope string) bool {
	return scope == "eventkit" || strings.HasPrefix(scope, "eventkit:")
}

// filterEventkit wraps a processor so that eventkit-scoped records are dropped,
// unless printEventkit is set.
func filterEventkit(next log.Processor, printEventkit bool) log.Processor {
	if printEventkit {
		return next
	}
	return scopeFilterProcessor{
		next:  next,
		allow: func(scope string) bool { return !isEventkitScope(scope) },
	}
}
