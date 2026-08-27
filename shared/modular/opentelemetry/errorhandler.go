// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// defaultErrorInterval is how often the same recurring OpenTelemetry error is
// printed. Export failures repeat once per batch interval (one second by
// default), which would otherwise bury every other line of output.
const defaultErrorInterval = time.Minute

// errorHandler is an otel.ErrorHandler that reports background OpenTelemetry
// errors -- most commonly an unreachable OTLP collector -- and lets the process
// continue. Identical consecutive messages are collapsed into one line per
// interval, together with the number of occurrences that were suppressed.
type errorHandler struct {
	mu       sync.Mutex
	w        io.Writer
	interval time.Duration
	now      func() time.Time

	lastMessage string
	lastPrinted time.Time
	suppressed  int
}

// newErrorHandler creates an errorHandler writing to w, printing a repeated
// message at most once per interval.
func newErrorHandler(w io.Writer, interval time.Duration) *errorHandler {
	return &errorHandler{
		w:        w,
		interval: interval,
		now:      time.Now,
	}
}

// Handle prints the error in the same format used by the pretty stdout
// exporter, so it does not stand out from the rest of the log output.
func (h *errorHandler) Handle(err error) {
	if err == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := h.now()
	message := err.Error()
	if message == h.lastMessage && now.Sub(h.lastPrinted) < h.interval {
		h.suppressed++
		return
	}

	suffix := ""
	if h.suppressed > 0 && message == h.lastMessage {
		suffix = fmt.Sprintf(" (%d identical errors suppressed)", h.suppressed)
	}

	_, _ = fmt.Fprintf(h.w, "%s\tERROR\totel\t%s%s\n", now.Format("15:04:05.000"), message, suffix)

	h.lastMessage = message
	h.lastPrinted = now
	h.suppressed = 0
}
