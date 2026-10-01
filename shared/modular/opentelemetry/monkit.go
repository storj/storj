// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// fieldAttribute is the attribute which holds the monkit field name (count, sum, ravg, ...)
// of the exported value.
const fieldAttribute = "field"

// monkitProducer is an OpenTelemetry metric producer, which exposes the current monkit values as
// gauges. Each monkit measurement becomes one metric, with the monkit tags and the field name as
// attributes.
type monkitProducer struct {
	registry *monkit.Registry
	filter   *metricFilter
	now      func() time.Time
}

// newMonkitProducer creates a producer for the given monkit registry.
func newMonkitProducer(registry *monkit.Registry, filter *metricFilter) *monkitProducer {
	return &monkitProducer{
		registry: registry,
		filter:   filter,
		now:      time.Now,
	}
}

// Produce implements metric.Producer.
func (p *monkitProducer) Produce(ctx context.Context) ([]metricdata.ScopeMetrics, error) {
	now := p.now()

	var names []string
	points := map[string][]metricdata.DataPoint[float64]{}

	p.registry.Stats(func(key monkit.SeriesKey, field string, val float64) {
		// OTLP receivers (and most backends) can't do anything useful with these, monkit
		// reports them for distributions without any observation.
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return
		}
		if !p.filter.include(key, field) {
			return
		}

		tags := key.Tags.All()
		attrs := make([]attribute.KeyValue, 0, len(tags)+1)
		for k, v := range tags {
			attrs = append(attrs, attribute.String(k, v))
		}
		attrs = append(attrs, attribute.String(fieldAttribute, field))

		if _, found := points[key.Measurement]; !found {
			names = append(names, key.Measurement)
		}
		points[key.Measurement] = append(points[key.Measurement], metricdata.DataPoint[float64]{
			Attributes: attribute.NewSet(attrs...),
			Time:       now,
			Value:      val,
		})
	})

	if len(names) == 0 {
		return nil, nil
	}

	metrics := make([]metricdata.Metrics, 0, len(names))
	for _, name := range names {
		metrics = append(metrics, metricdata.Metrics{
			Name: name,
			Data: metricdata.Gauge[float64]{
				DataPoints: points[name],
			},
		})
	}

	return []metricdata.ScopeMetrics{
		{
			Scope:   instrumentation.Scope{Name: "monkit"},
			Metrics: metrics,
		},
	}, nil
}

// metricFilter decides which monkit values are exported.
type metricFilter struct {
	excludedNames  map[string]struct{}
	excludedFields map[string]struct{}
	exceptions     []metricSelector
}

// newMetricFilter creates a filter from the metrics configuration.
func newMetricFilter(cfg Metrics) (*metricFilter, error) {
	exceptions, err := parseMetricSelectors(cfg.ExclusionException)
	if err != nil {
		return nil, err
	}
	return &metricFilter{
		excludedNames:  toSet(cfg.ExcludedNames),
		excludedFields: toSet(cfg.ExcludedFields),
		exceptions:     exceptions,
	}, nil
}

// include returns true, if the value should be exported.
func (f *metricFilter) include(key monkit.SeriesKey, field string) bool {
	for _, exception := range f.exceptions {
		if exception.matches(key, field) {
			return true
		}
	}
	if _, found := f.excludedNames[key.Measurement]; found {
		return false
	}
	if _, found := f.excludedFields[field]; found {
		return false
	}
	return true
}

// metricSelector matches monkit values by measurement name and tags, like `function_times{name="foobar"}`.
// The `field` tag matches the monkit field name.
type metricSelector struct {
	name string
	tags map[string]string
}

func (s metricSelector) matches(key monkit.SeriesKey, field string) bool {
	if s.name != key.Measurement {
		return false
	}
	if len(s.tags) == 0 {
		return true
	}
	tags := key.Tags.All()
	for k, v := range s.tags {
		if k == fieldAttribute {
			if field != v {
				return false
			}
			continue
		}
		actual, found := tags[k]
		if !found || actual != v {
			return false
		}
	}
	return true
}

// parseMetricSelectors parses a comma separated list of selectors, like
// `function_times{name="foo",scope="bar"},function{name="baz"}`.
func parseMetricSelectors(s string) (selectors []metricSelector, err error) {
	rest := strings.TrimSpace(s)
	for rest != "" {
		var selector metricSelector
		selector, rest, err = parseMetricSelector(rest)
		if err != nil {
			return nil, errs.New("invalid metric selector %q: %v", s, err)
		}
		selectors = append(selectors, selector)

		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		if rest[0] != ',' {
			return nil, errs.New("invalid metric selector %q: expected ',' before %q", s, rest)
		}
		rest = strings.TrimSpace(rest[1:])
	}
	return selectors, nil
}

// parseMetricSelector parses one selector from the beginning of s, and returns the unparsed remainder.
func parseMetricSelector(s string) (selector metricSelector, rest string, err error) {
	end := strings.IndexAny(s, "{,")
	if end < 0 {
		end = len(s)
	}
	selector.name = strings.TrimSpace(s[:end])
	if selector.name == "" {
		return selector, "", errs.New("missing metric name")
	}
	rest = s[end:]
	if !strings.HasPrefix(rest, "{") {
		return selector, rest, nil
	}
	rest = rest[1:]

	selector.tags = map[string]string{}
	for {
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "}") {
			return selector, rest[1:], nil
		}

		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			return selector, "", errs.New("missing '=' in tag definition")
		}
		key := strings.TrimSpace(rest[:eq])
		if key == "" {
			return selector, "", errs.New("missing tag name")
		}
		rest = strings.TrimSpace(rest[eq+1:])
		if !strings.HasPrefix(rest, `"`) {
			return selector, "", errs.New("value of tag %q should be quoted", key)
		}
		closing := strings.IndexByte(rest[1:], '"')
		if closing < 0 {
			return selector, "", errs.New("unterminated value of tag %q", key)
		}
		selector.tags[key] = rest[1 : closing+1]
		rest = strings.TrimSpace(rest[closing+2:])

		switch {
		case strings.HasPrefix(rest, ","):
			rest = rest[1:]
		case strings.HasPrefix(rest, "}"):
		default:
			return selector, "", errs.New("expected ',' or '}' after tag %q", key)
		}
	}
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			set[v] = struct{}{}
		}
	}
	return set
}
