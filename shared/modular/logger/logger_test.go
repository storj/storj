// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package logger

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
)

// captureProcessor collects every log Record it receives so tests can assert on
// what actually made it past the logger's level filter.
type captureProcessor struct {
	mu      sync.Mutex
	records []log.Record
}

func (p *captureProcessor) OnEmit(ctx context.Context, r *log.Record) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = append(p.records, *r)
	return nil
}

func (p *captureProcessor) Enabled(context.Context, log.EnabledParameters) bool { return true }
func (p *captureProcessor) ForceFlush(context.Context) error                    { return nil }
func (p *captureProcessor) Shutdown(context.Context) error                      { return nil }

func (p *captureProcessor) severities() []otellog.Severity {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]otellog.Severity, 0, len(p.records))
	for i := range p.records {
		out = append(out, p.records[i].Severity())
	}
	return out
}

func newTestProvider() (*captureProcessor, *log.LoggerProvider) {
	cap := &captureProcessor{}
	return cap, log.NewLoggerProvider(log.WithProcessor(cap))
}

func buildLogger(t *testing.T, level string, useOtelOnly bool) (*captureProcessor, RootLogger) {
	t.Helper()

	cfg := Config{
		Level:       level,
		Encoding:    "console",
		Output:      "stderr",
		UseOtelOnly: useOtelOnly,
	}
	zapCfg, err := NewZapConfig(cfg)
	require.NoError(t, err)

	cap, provider := newTestProvider()
	rl, err := NewRootLogger(cfg, zapCfg, provider)
	require.NoError(t, err)
	return cap, rl
}

func TestRootLogger_OtelOnly_RespectsLevel(t *testing.T) {
	cap, rl := buildLogger(t, "warn", true)

	rl.Debug("dbg")
	rl.Info("inf")
	rl.Warn("wrn")
	rl.Error("err")

	require.Equal(t, []otellog.Severity{
		otellog.SeverityWarn,
		otellog.SeverityError,
	}, cap.severities())
}

func TestRootLogger_Teed_OtelBranchRespectsLevel(t *testing.T) {
	cap, rl := buildLogger(t, "warn", false)

	rl.Debug("dbg")
	rl.Info("inf")
	rl.Warn("wrn")
	rl.Error("err")

	require.Equal(t, []otellog.Severity{
		otellog.SeverityWarn,
		otellog.SeverityError,
	}, cap.severities())
}

func TestRootLogger_OtelOnly_DebugLevel_LetsEverythingThrough(t *testing.T) {
	cap, rl := buildLogger(t, "debug", true)

	rl.Debug("dbg")
	rl.Info("inf")
	rl.Warn("wrn")
	rl.Error("err")

	require.Equal(t, []otellog.Severity{
		otellog.SeverityDebug,
		otellog.SeverityInfo,
		otellog.SeverityWarn,
		otellog.SeverityError,
	}, cap.severities())
}
