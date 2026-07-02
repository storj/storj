// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventkit

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"storj.io/eventkit"
	ekpb "storj.io/eventkit/pb"
)

// otelDestination is an eventkit.Destination that emits events as OpenTelemetry log records.
type otelDestination struct {
	logger log.Logger
}

// newOtelDestination creates a Destination that reports eventkit events through OpenTelemetry logging.
func newOtelDestination(provider *sdklog.LoggerProvider) *otelDestination {
	return &otelDestination{
		logger: provider.Logger("eventkit"),
	}
}

// Submit converts each event into an OpenTelemetry log record and emits it.
func (d *otelDestination) Submit(events ...*eventkit.Event) {
	ctx := context.Background()
	for _, event := range events {
		var record log.Record
		record.SetTimestamp(event.Timestamp)
		record.SetSeverity(log.SeverityInfo)
		record.SetEventName(event.Name)
		record.SetBody(log.StringValue(event.Name))
		record.AddAttributes(
			log.String("name", event.Name),
			log.String("scope", strings.Join(event.Scope, ".")),
		)
		for _, tag := range event.Tags {
			record.AddAttributes(tagToKeyValue(tag))
		}
		d.logger.Emit(ctx, record)
	}
}

// Run is a no-op: log records are emitted synchronously on Submit and the
// underlying LoggerProvider owns batching and flushing.
func (d *otelDestination) Run(ctx context.Context) {}

// tagToKeyValue converts an eventkit tag into an OpenTelemetry log attribute.
func tagToKeyValue(tag *ekpb.Tag) log.KeyValue {
	switch v := tag.Value.(type) {
	case *ekpb.Tag_String_:
		return log.String(tag.Key, string(v.String_))
	case *ekpb.Tag_Int64:
		return log.Int64(tag.Key, v.Int64)
	case *ekpb.Tag_Double:
		return log.Float64(tag.Key, v.Double)
	case *ekpb.Tag_Bytes:
		return log.Bytes(tag.Key, v.Bytes)
	case *ekpb.Tag_Bool:
		return log.Bool(tag.Key, v.Bool)
	case *ekpb.Tag_DurationNs:
		return log.Int64(tag.Key, v.DurationNs)
	case *ekpb.Tag_Timestamp:
		return log.Int64(tag.Key, v.Timestamp.Seconds*int64(1e9)+int64(v.Timestamp.Nanos))
	default:
		return log.Empty(tag.Key)
	}
}

var _ eventkit.Destination = (*otelDestination)(nil)
