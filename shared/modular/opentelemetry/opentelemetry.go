// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"os"

	"github.com/zeebo/errs"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Config is the configuration for the OpenTelemetry integration.
type Config struct {
	Logging Logging `flagname:"log"`
	Service string  `default:"storj" help:"OTel service name"`
}

// Logging is the configuration for OpenTelemetry log records.
type Logging struct {
	HTTPDestination string `default:"" help:"OTel HTTP destination for logs"`
	Stdout          string `default:"none" help:"stdout log format for OTel records: 'none' (disabled), 'json', or 'pretty'"`
	PrintEventkit   bool   `default:"false" help:"if true, eventkit events/logs are also printed to stdout (suppressed by default)"`
}

// Opentelemetry holds OpenTelemetry providers for logging, metrics, and tracing.
type Opentelemetry struct {
	Log *log.LoggerProvider
}

// NewOpentelemetry creates a new OpenTelemetry configuration with OTLP exporters.
func NewOpentelemetry(ctx context.Context, cfg Config) (*Opentelemetry, error) {
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
			return nil, errs.Wrap(err)
		}
		opts = append(opts, log.WithProcessor(log.NewBatchProcessor(exporter)))
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
		return nil, errs.New("invalid otel.log.stdout value %q (must be 'none', 'json', or 'pretty')", cfg.Logging.Stdout)
	}

	provider := log.NewLoggerProvider(opts...)

	return &Opentelemetry{
		Log: provider,
	}, nil
}
