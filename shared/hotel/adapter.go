// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hotel

import (
	"context"
	"strings"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Package returns a new Scope for the calling package.
func Package() *Scope {
	return ScopeNamed(CallerPackage(1))
}

// ScopeNamed returns a new Scope with the given name.
func ScopeNamed(name string) *Scope {
	tracer := otel.Tracer(name)
	meter := otel.Meter(name)
	return &Scope{
		tracer:  tracer,
		meter:   meter,
		sources: make(map[string]any),
	}
}

// Scope provides OpenTelemetry instrumentation for monkit.
type Scope struct {
	tracer  trace.Tracer
	meter   metric.Meter
	mtx     sync.RWMutex
	sources map[string]any
}

// Task creates a new monkit task for tracing.
func (s *Scope) Task(tags ...SeriesTag) monkit.Task {
	return func(ctx *context.Context, args ...interface{}) func(*error) {
		nctx, span := s.tracer.Start(*ctx, CallerFunc(0))
		*ctx = nctx
		return func(err *error) {
			if err != nil && *err != nil {
				span.RecordError(*err)
				span.SetStatus(codes.Error, "")
			}
			span.End()
		}
	}
}

// IntVal creates a new integer value gauge with the given name.
func (s *Scope) IntVal(name string, tags ...SeriesTag) *IntVal {
	iv := newSource[*IntVal](s, sourceName(name, tags), func() *IntVal {
		// errors are intentionally ignored to avoid flooding the log. Wrong names should be noticed during development.
		gauge, _ := s.meter.Int64Gauge(name)
		return &IntVal{
			gauge:  gauge,
			option: metric.WithAttributeSet(attribute.NewSet(toAttrs(tags)...)),
		}
	})
	return iv
}

// IntVal represents an OpenTelemetry integer gauge metric.
type IntVal struct {
	option metric.MeasurementOption
	gauge  metric.Int64Gauge
}

// Observe records a value for this integer gauge.
func (i *IntVal) Observe(value int64) {
	if i.gauge == nil {
		return
	}
	i.gauge.Record(context.Background(), value, i.option)
}

// FloatVal creates a new float value gauge with the given name.
func (s *Scope) FloatVal(name string, tags ...SeriesTag) *FloatVal {
	iv := newSource[*FloatVal](s, sourceName(name, tags), func() *FloatVal {
		// errors are intentionally ignored to avoid flooding the log. Wrong names should be noticed during development.
		gauge, _ := s.meter.Float64Gauge(name)
		return &FloatVal{
			gauge:  gauge,
			option: metric.WithAttributeSet(attribute.NewSet(toAttrs(tags)...)),
		}
	})
	return iv
}

// FloatVal represents an OpenTelemetry float64 gauge metric.
type FloatVal struct {
	gauge  metric.Float64Gauge
	option metric.MeasurementOption
}

// Observe records a value for this float gauge.
func (d *FloatVal) Observe(val float64) {
	if d.gauge == nil {
		return
	}
	d.gauge.Record(context.Background(), val, d.option)
}

func toAttrs(tags []SeriesTag) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(tags))
	for _, t := range tags {
		out = append(out, attribute.String(t.Key, t.Val))
	}
	return out
}

func newSource[T any](s *Scope, name string, constructor func() T) (rv T) {
	s.mtx.RLock()
	source, exists := s.sources[name]
	s.mtx.RUnlock()

	if exists {
		s, _ := source.(T)
		return s
	}

	s.mtx.Lock()
	if source, exists := s.sources[name]; exists {
		s.mtx.Unlock()
		s, _ := source.(T)
		return s
	}

	ss := constructor()
	s.sources[name] = ss
	s.mtx.Unlock()

	return ss
}

func sourceName(name string, tags []SeriesTag) string {
	var sourceNameSize int
	sourceNameSize += len(name) + len(tags)*2
	for _, tag := range tags {
		sourceNameSize += len(tag.Key) + len(tag.Val)
	}
	var sourceName strings.Builder
	sourceName.Grow(sourceNameSize)
	sourceName.WriteString(name)
	for _, tag := range tags {
		sourceName.WriteByte(',')
		sourceName.WriteString(tag.Key)
		sourceName.WriteByte('=')
		sourceName.WriteString(tag.Val)
	}
	return sourceName.String()
}
