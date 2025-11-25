// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package motel

import (
	"context"
	"encoding/binary"

	"github.com/spacemonkeygo/monkit/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TraceProvider implements OpenTelemetry TracerProvider using monkit.
type TraceProvider struct {
	noop.TracerProvider
	registry *monkit.Registry
}

// NewTraceProvider creates a new TraceProvider with the given monkit registry.
func NewTraceProvider(registry *monkit.Registry) *TraceProvider {
	return &TraceProvider{
		registry: registry,
	}
}

// Tracer returns a new tracer with the given name.
func (t *TraceProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	return &MonkitTracer{
		scope:    t.registry.ScopeNamed(name),
		provider: t,
	}
}

var _ trace.TracerProvider = &TraceProvider{}

// MonkitTracer implements OpenTelemetry Tracer using monkit.
type MonkitTracer struct {
	noop.Tracer
	scope    *monkit.Scope
	provider *TraceProvider
}

// Start creates a new span with the given name.
func (m *MonkitTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	f := m.scope.FuncNamed(spanName)
	task := f.Task(&ctx)
	return ctx, &MonkitSpan{
		span:     monkit.SpanFromCtx(ctx),
		task:     task,
		provider: m.provider,
	}
}

var _ trace.Tracer = &MonkitTracer{}

// MonkitSpan implements OpenTelemetry Span using monkit.
type MonkitSpan struct {
	noop.Span
	span     *monkit.Span
	task     func(*error)
	provider *TraceProvider
	status   codes.Code
	err      error
}

// End completes the span.
func (m *MonkitSpan) End(options ...trace.SpanEndOption) {
	m.task(&m.err)
}

// RecordError records an error for this span.
func (m *MonkitSpan) RecordError(err error, options ...trace.EventOption) {
	m.err = err
}

// SpanContext returns the span context for this span.
func (m *MonkitSpan) SpanContext() trace.SpanContext {
	scc := trace.SpanContextConfig{
		TraceID: toOtelTraceID(m.span.Trace().Id()),
		SpanID:  toOtelSpanID(m.span.Id()),
	}
	return trace.NewSpanContext(scc)

}

func toOtelTraceID(id int64) trace.TraceID {
	b := [16]byte{}
	binary.BigEndian.PutUint64(b[:], uint64(id))
	return b
}

func toOtelSpanID(id int64) trace.SpanID {
	b := [8]byte{}
	binary.BigEndian.PutUint64(b[:], uint64(id))
	return b
}

// SetStatus sets the status code and description for this span.
func (m *MonkitSpan) SetStatus(code codes.Code, description string) {
	m.status = code
}

// SetAttributes sets attributes on this span.
func (m *MonkitSpan) SetAttributes(kvs ...attribute.KeyValue) {
	for _, kv := range kvs {
		if kv.Value.Type() == attribute.STRING {
			m.span.Annotate(string(kv.Key), kv.Value.AsString())
		}
	}
}

// TracerProvider returns the tracer provider for this span.
func (m *MonkitSpan) TracerProvider() trace.TracerProvider {
	return m.provider
}

var _ trace.Span = &MonkitSpan{}
