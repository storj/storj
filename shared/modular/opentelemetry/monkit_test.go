// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

func TestMonkitProducer(t *testing.T) {
	registry := monkit.NewRegistry()
	scope := registry.ScopeNamed("test")
	scope.Counter("requests", monkit.NewSeriesTag("kind", "upload")).Inc(3)
	scope.IntVal("size").Observe(10)
	scope.IntVal("size").Observe(20)

	filter, err := newMetricFilter(Metrics{ExcludedFields: []string{"ravg"}})
	require.NoError(t, err)

	producer := newMonkitProducer(registry, filter)
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	producer.now = func() time.Time { return now }

	scopeMetrics, err := producer.Produce(context.Background())
	require.NoError(t, err)
	require.Len(t, scopeMetrics, 1)

	metrics := map[string]metricdata.Gauge[float64]{}
	for _, m := range scopeMetrics[0].Metrics {
		gauge, ok := m.Data.(metricdata.Gauge[float64])
		require.True(t, ok)
		metrics[m.Name] = gauge
	}
	require.Contains(t, metrics, "requests")
	require.Contains(t, metrics, "size")

	values := func(gauge metricdata.Gauge[float64]) map[string]float64 {
		result := map[string]float64{}
		for _, point := range gauge.DataPoints {
			require.Equal(t, now, point.Time)
			field, ok := point.Attributes.Value(fieldAttribute)
			require.True(t, ok)
			result[field.AsString()] = point.Value
		}
		return result
	}

	requests := metrics["requests"]
	require.Equal(t, float64(3), values(requests)["value"])
	kind, ok := requests.DataPoints[0].Attributes.Value("kind")
	require.True(t, ok)
	require.Equal(t, "upload", kind.AsString())
	scopeTag, ok := requests.DataPoints[0].Attributes.Value("scope")
	require.True(t, ok)
	require.Equal(t, "test", scopeTag.AsString())

	size := values(metrics["size"])
	require.Equal(t, float64(2), size["count"])
	require.Equal(t, float64(30), size["sum"])
	require.NotContains(t, size, "ravg")
}

func TestMonkitProducerEmpty(t *testing.T) {
	filter, err := newMetricFilter(Metrics{})
	require.NoError(t, err)

	scopeMetrics, err := newMonkitProducer(monkit.NewRegistry(), filter).Produce(context.Background())
	require.NoError(t, err)
	require.Empty(t, scopeMetrics)
}

func TestMetricFilter(t *testing.T) {
	filter, err := newMetricFilter(Metrics{
		ExcludedNames:      []string{"function_times", "function"},
		ExcludedFields:     []string{"ravg", "max"},
		ExclusionException: `function_times{name="foobar"}, function{name="baz",field="total"}`,
	})
	require.NoError(t, err)

	foobar := monkit.NewSeriesKey("function_times").WithTag("name", "foobar")
	other := monkit.NewSeriesKey("function_times").WithTag("name", "other")
	baz := monkit.NewSeriesKey("function").WithTag("name", "baz")
	size := monkit.NewSeriesKey("size")

	for _, tc := range []struct {
		key      monkit.SeriesKey
		field    string
		included bool
	}{
		{key: foobar, field: "count", included: true},
		{key: foobar, field: "ravg", included: true},
		{key: other, field: "count", included: false},
		{key: baz, field: "total", included: true},
		{key: baz, field: "errors", included: false},
		{key: size, field: "count", included: true},
		{key: size, field: "ravg", included: false},
		{key: size, field: "max", included: false},
	} {
		require.Equal(t, tc.included, filter.include(tc.key, tc.field), "%s %s", tc.key, tc.field)
	}
}

func TestParseMetricSelectors(t *testing.T) {
	selectors, err := parseMetricSelectors("")
	require.NoError(t, err)
	require.Empty(t, selectors)

	selectors, err = parseMetricSelectors(`size, function_times{name="a,b}", scope = "x" }, function{}`)
	require.NoError(t, err)
	require.Equal(t, []metricSelector{
		{name: "size"},
		{name: "function_times", tags: map[string]string{"name": "a,b}", "scope": "x"}},
		{name: "function", tags: map[string]string{}},
	}, selectors)

	for _, invalid := range []string{
		`{name="a"}`,
		`function_times{name}`,
		`function_times{name=a}`,
		`function_times{name="a`,
		`function_times{name="a" scope="b"}`,
		`function_times{name="a"} size`,
		`function_times{="a"}`,
	} {
		_, err := parseMetricSelectors(invalid)
		require.Error(t, err, invalid)
	}
}

func TestMeterProviderExport(t *testing.T) {
	requests := make(chan *collectormetrics.ExportMetricsServiceRequest, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var request collectormetrics.ExportMetricsServiceRequest
		if err := proto.Unmarshal(body, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- &request
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(nil)
	}))
	defer server.Close()

	monkit.Default.ScopeNamed("otel-test").Counter("exported_counter").Inc(42)

	ctx := context.Background()
	provider, err := newMeterProvider(ctx, Metrics{
		HTTPDestination: strings.TrimPrefix(server.URL, "http://"),
		Interval:        time.Hour,
	}, resource.Empty(), newErrorHandler(io.Discard, time.Minute))
	require.NoError(t, err)
	require.NotNil(t, provider)

	require.NoError(t, provider.ForceFlush(ctx))
	require.NoError(t, provider.Shutdown(ctx))

	var found bool
	for len(requests) > 0 {
		request := <-requests
		for _, rm := range request.ResourceMetrics {
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					if m.Name != "exported_counter" {
						continue
					}
					for _, point := range m.GetGauge().GetDataPoints() {
						if point.GetAsDouble() == 42 {
							found = true
						}
					}
				}
			}
		}
	}
	require.True(t, found, "exported_counter is not received by the collector")
}

func TestMeterProviderUnavailableCollector(t *testing.T) {
	// reserve a port, and close it immediately, so nobody listens on it.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	ctx := context.Background()
	provider, err := newMeterProvider(ctx, Metrics{
		HTTPDestination: address,
		Interval:        time.Hour,
	}, resource.Empty(), newErrorHandler(io.Discard, time.Minute))
	require.NoError(t, err)

	// the export is retried until the deadline, and the values are discarded after that.
	flushCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	require.Error(t, provider.ForceFlush(flushCtx))

	otel := &Opentelemetry{Metric: provider, errorHandler: newErrorHandler(io.Discard, time.Minute)}
	closeCtx, cancelClose := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancelClose()
	require.NoError(t, otel.Close(closeCtx))
}

func TestMeterProviderDisabled(t *testing.T) {
	provider, err := newMeterProvider(context.Background(), Metrics{}, resource.Empty(), newErrorHandler(io.Discard, time.Minute))
	require.NoError(t, err)
	require.Nil(t, provider)

	require.NoError(t, (&Opentelemetry{}).Close(context.Background()))
}
