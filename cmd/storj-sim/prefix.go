// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

// PrefixWriter writes to the specified output with prefixes.
type PrefixWriter struct {
	root    *prefixWriter
	maxline int

	mu        sync.Mutex
	prefixlen int
	dst       io.Writer
}

const maxIDLength = 10

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// NewPrefixWriter creates a writer than can prefix all lines written to it.
func NewPrefixWriter(defaultPrefix string, dst io.Writer) *PrefixWriter {
	writer := &PrefixWriter{
		maxline: 10000, // disable maxline cutting
		dst:     dst,
	}
	writer.root = writer.Prefixed(defaultPrefix).(*prefixWriter)
	return writer
}

// prefixWriter is the implementation that handles buffering and prefixing.
type prefixWriter struct {
	*PrefixWriter
	prefix string
	id     string
	buffer []byte
}

// Prefixed returns a new writer that has writes with specified prefix.
func (writer *PrefixWriter) Prefixed(prefix string) io.Writer {
	writer.mu.Lock()
	writer.prefixlen = max(writer.prefixlen, len(prefix))
	writer.mu.Unlock()

	return &prefixWriter{writer, prefix, "", make([]byte, 0, writer.maxline)}
}

// Write implements io.Writer that prefixes lines.
func (writer *PrefixWriter) Write(data []byte) (int, error) {
	return writer.root.Write(data)
}

// Write implements io.Writer that prefixes lines
func (writer *prefixWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	var newID string
	if writer.id == "" {
		if start := bytes.Index(data, []byte("Node ")); start > 0 {
			if end := bytes.Index(data[start:], []byte(" started")); end > 0 {
				newID = string(data[start+5 : start+end])
				if len(newID) > maxIDLength {
					newID = newID[:maxIDLength]
				}
			}
		}
	}

	buffer := data

	// buffer everything that hasn't been written yet
	if len(writer.buffer) > 0 {
		buffer = append(writer.buffer, data...)
		defer func() {
			writer.buffer = buffer
		}()
	} else {
		defer func() {
			if len(buffer) > 0 {
				writer.buffer = append(writer.buffer, buffer...)
			}
		}()
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()

	if newID != "" {
		writer.id = newID
	}

	prefix := writer.prefix
	id := writer.id
	timeText := time.Now().Format("15:04:05.000")
	for len(buffer) > 0 {
		pos := bytes.IndexByte(buffer, '\n') + 1
		breakline := false
		if pos <= 0 {
			if len(buffer) < writer.maxline {
				return len(data), nil
			}
		}
		if pos < 0 || pos > writer.maxline {
			pos = writer.maxline
			for p := pos; p >= writer.maxline*2/3; p-- {
				if buffer[p] == ' ' {
					pos = p
					break
				}
			}
			breakline = true
		}

		_, err := fmt.Fprintf(writer.dst, "%-*s %-*s %s | ", writer.prefixlen, prefix, maxIDLength, id, timeText)
		if err != nil {
			return len(data), err
		}

		_, err = writer.dst.Write(buffer[:pos])
		buffer = buffer[pos:]

		if err != nil {
			return len(data), err
		}

		if breakline {
			_, err = writer.dst.Write([]byte{'\n'})
			if err != nil {
				return len(data), err
			}
		}

		prefix = ""
		id = ""
		timeText = "            "
	}

	return len(data), nil
}
