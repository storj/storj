// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package motel

import (
	"context"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// MeterProvider implements OpenTelemetry MeterProvider using monkit.
type MeterProvider struct {
	noop.MeterProvider
	registry *monkit.Registry
}

// NewMeterProvider creates a new MeterProvider with the given monkit registry.
func NewMeterProvider(registry *monkit.Registry) *MeterProvider {
	return &MeterProvider{
		registry: registry,
	}
}

// Meter returns a new meter with the given name.
func (m *MeterProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return &Meter{
		scope: m.registry.ScopeNamed(name),
	}
}

var _ metric.MeterProvider = &MeterProvider{}

// Meter implements OpenTelemetry Meter using monkit.
type Meter struct {
	noop.Meter
	scope *monkit.Scope
}

// Int64UpDownCounter creates a new up/down counter with the given name.
func (m *Meter) Int64UpDownCounter(name string, option ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	return &Int64UpDownCounter{
		name:  name,
		scope: m.scope,
	}, nil
}

// Int64UpDownCounter implements OpenTelemetry Int64UpDownCounter using monkit.
type Int64UpDownCounter struct {
	noop.Int64UpDownCounter
	scope *monkit.Scope
	mu    sync.Mutex
	value *monkit.Counter
	name  string
}

// Add increments the counter by the given amount.
func (i *Int64UpDownCounter) Add(ctx context.Context, incr int64, options ...metric.AddOption) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.value == nil {
		i.value = i.scope.Counter(i.name, attributesToTags(metric.NewAddConfig(options).Attributes())...)
	}
	i.value.Inc(incr)
}

// Int64Gauge creates a new int64 gauge with the given name.
func (m *Meter) Int64Gauge(name string, option ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return &Int64Gauge{
		name:  name,
		scope: m.scope,
	}, nil
}

// Int64Gauge implements OpenTelemetry Int64Gauge using monkit.
type Int64Gauge struct {
	noop.Int64Gauge
	scope *monkit.Scope
	mu    sync.Mutex
	value *monkit.IntVal
	name  string
}

// Record records a value for this gauge.
func (f *Int64Gauge) Record(ctx context.Context, value int64, options ...metric.RecordOption) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.value == nil {
		f.value = f.scope.IntVal(f.name, attributesToTags(metric.NewRecordConfig(options).Attributes())...)
	}
	f.value.Observe(value)
}

// Float64Gauge creates a new float64 gauge with the given name.
func (m *Meter) Float64Gauge(name string, options ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	return &Float64Gauge{
		name:  name,
		scope: m.scope,
	}, nil
}

// Float64Gauge implements OpenTelemetry Float64Gauge using monkit.
type Float64Gauge struct {
	noop.Float64Gauge
	scope *monkit.Scope
	mu    sync.Mutex
	value *monkit.FloatVal
	name  string
}

// Record records a value for this gauge.
func (f *Float64Gauge) Record(ctx context.Context, value float64, options ...metric.RecordOption) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.value == nil {
		f.value = f.scope.FloatVal(f.name, attributesToTags(metric.NewRecordConfig(options).Attributes())...)
	}
	f.value.Observe(value)
}

func attributesToTags(attrs attribute.Set) []monkit.SeriesTag {
	var tags []monkit.SeriesTag
	for i := 0; i < attrs.Len(); i++ {
		kv, found := attrs.Get(i)
		if !found {
			continue
		}
		if kv.Value.Type() == attribute.STRING {
			tags = append(tags, monkit.NewSeriesTag(string(kv.Key), kv.Value.AsString()))
		}

	}
	return tags
}

var _ metric.Meter = &Meter{}
