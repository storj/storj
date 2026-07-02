// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"

	"storj.io/eventkit"
	"storj.io/storj/shared/modular"
	eventkit2 "storj.io/storj/shared/modular/eventkit"
)

var ek = eventkit.Package()

// LogTest is a helper component that emits log and eventkit records for testing log destinations.
type LogTest struct {
	log    *zap.Logger
	cancel *modular.StopTrigger
	olog   *log.LoggerProvider
}

// NewLogTest creates a new LogTest.
func NewLogTest(log *zap.Logger, cancel *modular.StopTrigger, olog *log.LoggerProvider, ek *eventkit2.Eventkit) *LogTest {
	return &LogTest{
		olog:   olog,
		log:    log,
		cancel: cancel,
	}
}

// Run emits a series of eventkit events and then triggers a shutdown.
func (l *LogTest) Run(ctx context.Context) error {
	for i := 0; i < 1000; i++ {
		// l.log.Info("zaplog", zap.Int("i", i))
		// l.log.Debug("zaplog-debug", zap.Int("i", i))
		ek.Event("ekevent", eventkit.String("foo", "bar"), eventkit.Int64("index", int64(i)), eventkit.Timestamp("now", time.Now()))
		// var record log.Record
		// record.SetTimestamp(time.Now())
		// record.SetSeverity(log.SeverityInfo)
		// record.SetEventName("my-event-name")
		// record.SetBody(log.StringValue("my-log-boddy"))
		// (*l.olog).Logger("tester").Emit(ctx, record)
		time.Sleep(time.Second)
	}

	l.cancel.Cancel()
	return nil
}
