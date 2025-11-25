// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hotel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.opentelemetry.io/otel"
	"golang.org/x/exp/slices"

	"storj.io/storj/shared/motel"
)

func TestFloatVal(t *testing.T) {
	CompareMetrics(t, "floattest", func(t *testing.T, m *monkit.Scope, h *Scope, rm func() []reportedMetric, rh func() []reportedMetric) {
		valm := m.FloatVal("foo", monkit.NewSeriesTag("foo", "bar"))
		valm.Observe(12.34)
		valm.Observe(12.34)

		valh := h.FloatVal("foo", NewSeriesTag("foo", "bar"))
		valh.Observe(12.34)
		valh.Observe(12.34)

		mustEqual(t, rm(), rh())
	})
}

func TestIntVal(t *testing.T) {
	CompareMetrics(t, "floattest", func(t *testing.T, m *monkit.Scope, h *Scope, rm func() []reportedMetric, rh func() []reportedMetric) {
		valm := m.IntVal("foo", monkit.NewSeriesTag("foo", "bar"))
		valm.Observe(12)
		valm.Observe(1)

		valh := h.IntVal("foo", NewSeriesTag("foo", "bar"))
		valh.Observe(12)
		valh.Observe(1)

		mustEqual(t, rm(), rh())
	})
}

func TestTraces(t *testing.T) {
	CompareTraces(t, "tracetest", func(t *testing.T, m *monkit.Scope, h *Scope, rm func() []reportedMetric, rh func() []reportedMetric) {
		ctx := context.Background()
		defer m.Task()(&ctx)(nil)
		defer h.Task()(&ctx)(nil)
		err := func1(ctx, m, h)
		require.Error(t, err)

		normalize := func(metrics []reportedMetric) []reportedMetric {
			var normalized []reportedMetric
			for _, metric := range metrics {
				if strings.HasPrefix(metric.key, "function_times") {
					metric.val = 0
				}
				normalized = append(normalized, metric)
			}
			return normalized
		}

		mustEqual(t, normalize(rm()), normalize(rh()))

	})
}

func mustEqual(t *testing.T, o []reportedMetric, n []reportedMetric) {
	for _, metric := range o {
		fmt.Println(metric)
	}
	fmt.Println("----")
	for _, metric := range n {
		fmt.Println(metric)
	}

	require.Equal(t, len(o), len(n))
	for i := 0; i < len(o); i++ {
		require.Equal(t, o[i].key, n[i].key)
		require.Equal(t, o[i].field, n[i].field)
		require.Equal(t, o[i].val, n[i].val)
	}

}

func func1(ctx context.Context, s1 *monkit.Scope, s2 *Scope) (err error) {
	defer s1.Task()(&ctx)(&err)
	defer s2.Task()(&ctx)(&err)
	return func2(ctx, s1, s2)
}

func func2(ctx context.Context, s1 *monkit.Scope, s2 *Scope) (err error) {
	defer s1.Task()(&ctx)(&err)
	defer s2.Task()(&ctx)(&err)
	err = errs.New("some error")
	return err
}

func CompareMetrics(t *testing.T, name string, cb func(t *testing.T, sm *monkit.Scope, sh *Scope, rm func() []reportedMetric, rh func() []reportedMetric)) {
	rm := monkit.NewRegistry()
	rh := monkit.NewRegistry()
	m := rm.ScopeNamed(name)
	orig := otel.GetMeterProvider()
	otel.SetMeterProvider(motel.NewMeterProvider(rh))
	defer func() {
		otel.SetMeterProvider(orig)
	}()
	h := ScopeNamed(name)
	cb(t, m, h, func() []reportedMetric {
		return collect(rm)
	}, func() []reportedMetric {
		return collect(rh)
	})
}

func CompareTraces(t *testing.T, name string, cb func(t *testing.T, sm *monkit.Scope, sh *Scope, rm func() []reportedMetric, rh func() []reportedMetric)) {
	rm := monkit.NewRegistry()
	rh := monkit.NewRegistry()
	m := rm.ScopeNamed(name)
	orig := otel.GetTracerProvider()
	otel.SetTracerProvider(motel.NewTraceProvider(rh))
	defer func() {
		otel.SetTracerProvider(orig)
	}()
	h := ScopeNamed(name)
	cb(t, m, h, func() []reportedMetric {
		return collect(rm)
	}, func() []reportedMetric {
		return collect(rh)
	})
}

func collect(rh *monkit.Registry) []reportedMetric {
	var results []reportedMetric
	rh.Stats(func(key monkit.SeriesKey, field string, val float64) {
		results = append(results, reportedMetric{
			key:   normalizeKey(key.String()),
			field: field,
			val:   val,
		})
	})
	slices.SortFunc(results, func(a, b reportedMetric) int {
		c := strings.Compare(a.key, b.key)
		if c != 0 {
			return c
		}
		f := strings.Compare(a.field, b.field)
		if f != 0 {
			return f
		}
		if a.val < b.val {
			return -1
		}
		if a.val > b.val {
			return 1
		}
		return 0
	})
	return results
}

func normalizeKey(s string) string {
	parts := strings.Split(s, ",")
	sort.Strings(parts[1:])
	return strings.Join(parts, ",")
}

type reportedMetric struct {
	key   string
	field string
	val   float64
}
