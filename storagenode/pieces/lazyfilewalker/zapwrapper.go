// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapWrapper struct {
	Log *zap.Logger
}

var _ io.Writer = (*zapWrapper)(nil)

// Write writes the provided bytes to the underlying logger
// returns the length of the bytes.
//
// Write will split the input on newlines, parse, and post each line as a new log entry
// to the logger.
func (w *zapWrapper) Write(p []byte) (n int, err error) {
	n = len(p)
	for len(p) > 0 {
		p, err = w.writeLine(p)
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

type zapLogger struct {
	Caller  string        `json:"C"`
	Level   zapcore.Level `json:"L"`
	Message string        `json:"M"`
	Stack   string        `json:"S"`
	Name    string        `json:"N"`
	Time    time.Time     `json:"T"`

	LogMap map[string]interface{} `json:"-"`
}

// writeLine writes a single line from the input and returns the remaining bytes.
func (w *zapWrapper) writeLine(b []byte) (remaining []byte, err error) {
	idx := bytes.IndexByte(b, '\n')
	if idx < 0 {
		// If there are no newlines, log the entire string.
		return nil, w.log(b)
	}
	// Split on the newline, log the left.
	b, remaining = b[:idx], b[idx+1:]

	return remaining, w.log(b)
}

func (w *zapWrapper) log(b []byte) error {
	logger := zapLogger{}
	if err := json.Unmarshal(b, &logger); err != nil {
		return err
	}
	// parse the unknown fields into a map
	if err := json.Unmarshal(b, &logger.LogMap); err != nil {
		return err
	}
	// remove the known fields that are already parsed from the map
	delete(logger.LogMap, "C")
	delete(logger.LogMap, "L")
	delete(logger.LogMap, "M")
	delete(logger.LogMap, "S")
	delete(logger.LogMap, "N")
	delete(logger.LogMap, "T")

	log := w.Log.Named(logger.Name)
	if ce := log.Check(logger.Level, logger.Message); ce != nil {
		if logger.Stack != "" {
			ce.Stack = logger.Stack
		}
		if caller := newEntryCaller(logger.Caller); caller != nil {
			ce.Caller = *caller
		}

		if !logger.Time.IsZero() {
			ce.Time = logger.Time
		}

		var fields []zapcore.Field
		for key, val := range logger.LogMap {
			fields = append(fields, zap.Any(key, val))
		}

		ce.Write(fields...)
	}

	return nil
}

func newEntryCaller(caller string) *zapcore.EntryCaller {
	if caller == "" {
		return nil
	}

	idx := strings.IndexByte(caller, ':')
	if idx <= 0 {
		return nil
	}

	file, line := caller[:idx], caller[idx+1:]
	lineNum, err := strconv.Atoi(line)
	if err != nil {
		return nil
	}
	entryCaller := zapcore.NewEntryCaller(0, file, lineNum, true)
	return &entryCaller
}
