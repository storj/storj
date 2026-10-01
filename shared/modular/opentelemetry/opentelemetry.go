// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"os"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Config is the configuration for the OpenTelemetry integration.
type Config struct {
	Metrics Metrics `flagname:"metrics"`
	Logging Logging `flagname:"log"`
	Service string  `default:"storj" help:"OTel service name"`
}

// Logging is the configuration for OpenTelemetry log records.
type Logging struct {
	HTTPDestination string `default:"" help:"OTel HTTP destination for logs"`
	Stdout          string `default:"none" help:"stdout log format for OTel records: 'none' (disabled), 'json', or 'pretty'"`
	PrintEventkit   bool   `default:"false" help:"if true, eventkit events/logs are also printed to stdout (suppressed by default)"`
}

// Metrics is the configuration for exporting monkit metrics as OpenTelemetry metrics.
type Metrics struct {
	HTTPDestination    string        `default:"" help:"OTel HTTP destination for monkit metrics"`
	Interval           time.Duration `default:"1m" help:"how often monkit metrics are exported"`
	ExcludedNames      []string      `default:"function_times,function" help:"monkit measurements which are not exported"`
	ExcludedFields     []string      `default:"ravg,r99,r95,r90,r50,r10,rmin,rmax,min,max" help:"monkit fields which are not exported"`
	ExclusionException string        `default:"" help:"comma separated list of metrics to be exported, even if they are excluded by other rules (example: function_times{name=\"foobar\",field=\"count\"})"`
}

// Opentelemetry holds OpenTelemetry providers for logging, metrics, and tracing.
type Opentelemetry struct {
	Log    *log.LoggerProvider
	Metric *metric.MeterProvider

	errorHandler *errorHandler
}

// NewOpentelemetry creates a new OpenTelemetry configuration with OTLP exporters.
func NewOpentelemetry(ctx context.Context, cfg Config) (*Opentelemetry, error) {
	// Export failures (typically a collector that is down) are reported by the SDK
	// through the global error handler. Route them to stderr in the usual log
	// format instead of the SDK default, which uses the bare stdlib logger.
	errorHandler := newErrorHandler(os.Stderr, defaultErrorInterval)
	otel.SetErrorHandler(errorHandler)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.Service),
		),
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	opts := []log.LoggerProviderOption{
		log.WithResource(res),
	}

	// When no exporter is configured, register no processor at all: the provider
	// then silently drops every record. This is the SDK-native way of a noop
	// output; there is no exported noop exporter to plug in.
	if cfg.Logging.HTTPDestination != "" {
		exporter, err := otlploghttp.New(ctx,
			otlploghttp.WithInsecure(),
			otlploghttp.WithEndpoint(cfg.Logging.HTTPDestination),
		)
		if err != nil {
			// a broken log destination must never stop the process from starting:
			// report it and keep running without OTLP export.
			errorHandler.Handle(errs.New("OTLP log export to %q is disabled, exporter could not be created: %v",
				cfg.Logging.HTTPDestination, err))
		} else {
			opts = append(opts, log.WithProcessor(log.NewBatchProcessor(exporter)))
		}
	}

	switch cfg.Logging.Stdout {
	case "", "none":
		// stdout logging disabled.
	case "json", "pretty":
		var exporter log.Exporter
		if cfg.Logging.Stdout == "pretty" {
			exporter = newPrettyExporter(os.Stdout)
		} else {
			exporter, err = stdoutlog.New()
			if err != nil {
				return nil, errs.Wrap(err)
			}
		}
		// use a simple processor so records show up immediately and in order on
		// the console, and drop eventkit records unless explicitly requested.
		processor := filterEventkit(log.NewSimpleProcessor(exporter), cfg.Logging.PrintEventkit)
		opts = append(opts, log.WithProcessor(processor))
	default:
		return nil, errs.New("invalid otel.logging.stdout value %q (must be 'none', 'json', or 'pretty')", cfg.Logging.Stdout)
	}

	provider := log.NewLoggerProvider(opts...)

	meterProvider, err := newMeterProvider(ctx, cfg.Metrics, res, errorHandler)
	if err != nil {
		return nil, err
	}

	return &Opentelemetry{
		Log:    provider,
		Metric: meterProvider,

		errorHandler: errorHandler,
	}, nil
}

// newMeterProvider creates a meter provider which periodically exports the monkit metrics.
// Returns nil if the export is not configured.
func newMeterProvider(ctx context.Context, cfg Metrics, res *resource.Resource, errorHandler *errorHandler) (*metric.MeterProvider, error) {
	if cfg.HTTPDestination == "" {
		return nil, nil
	}

	filter, err := newMetricFilter(cfg)
	if err != nil {
		return nil, err
	}

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithEndpoint(cfg.HTTPDestination),
	)
	if err != nil {
		// a broken metric destination must never stop the process from starting.
		errorHandler.Handle(errs.New("OTLP metric export to %q is disabled, exporter could not be created: %v",
			cfg.HTTPDestination, err))
		return nil, nil
	}

	reader := metric.NewPeriodicReader(exporter,
		metric.WithInterval(cfg.Interval),
		metric.WithProducer(newMonkitProducer(monkit.Default, filter)),
	)
	return metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	), nil
}

// Close stops the metric export, after a final export of the current values.
func (o *Opentelemetry) Close(ctx context.Context) error {
	if o.Metric == nil {
		return nil
	}
	// a failing final export (collector is not available) is not a reason to fail the shutdown.
	o.errorHandler.Handle(o.Metric.Shutdown(ctx))
	return nil
}
