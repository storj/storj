// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var useAbsTime = os.Getenv("STORJ_TESTPLANET_ABSTIME")

// NewLogger creates a zaptest logger with nice defaults for tests.
func NewLogger(t *testing.T) *zap.Logger {
	if useAbsTime != "" {
		return zaptest.NewLogger(t)
	}

	start := time.Now()
	cfg := zap.NewDevelopmentEncoderConfig()

	cfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		var nanos, seconds, minutes int64
		nanos = t.Sub(start).Nanoseconds()

		seconds, nanos = nanos/1e9, nanos%1e9
		minutes, seconds = seconds/60, seconds%60

		enc.AppendString(fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, nanos/1e6))
	}

	enc := zapcore.NewConsoleEncoder(cfg)
	writer := newTestingWriter(t)
	level := zapcore.DebugLevel

	if l := os.Getenv("STORJ_TEST_LOG_LEVEL"); l != "" {
		_ = level.Set(l)
	}

	return zap.New(
		zapcore.NewCore(enc, writer, level),
		zap.ErrorOutput(writer.WithMarkFailed(true)),
	)
}

type testingWriter struct {
	t          *testing.T
	markFailed bool
}

func newTestingWriter(t *testing.T) testingWriter {
	return testingWriter{t: t}
}

func (w testingWriter) WithMarkFailed(v bool) testingWriter {
	w.markFailed = v
	return w
}

func (w testingWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	p = bytes.TrimRight(p, "\n")
	w.t.Logf("%s", p)
	if w.markFailed {
		w.t.Fail()
	}
	return n, nil
}

func (w testingWriter) Sync() error { return nil }
