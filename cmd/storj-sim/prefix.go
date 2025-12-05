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
	nowFunc func() time.Time

	mu        sync.Mutex
	prefixlen int
	dst       io.Writer
}

const (
	maxIDLength    = 10
	timeFormat     = "15:04:05.000"
	emptyTimeField = "            "
)

// NewPrefixWriter creates a writer than can prefix all lines written to it.
func NewPrefixWriter(defaultPrefix string, maxLineLen int, dst io.Writer) *PrefixWriter {
	writer := &PrefixWriter{
		maxline: maxLineLen,
		dst:     dst,
		nowFunc: time.Now,
	}
	writer.root = writer.Prefixed(defaultPrefix).(*prefixWriter)
	return writer
}

// prefixWriter is the implementation that handles buffering and prefixing.
type prefixWriter struct {
	*PrefixWriter
	prefix string

	local  sync.Mutex
	id     string
	buffer []byte
}

// WriterFlusher implements io.Writer and flushing of pending content.
type WriterFlusher interface {
	io.Writer
	Flush() error
}

// Prefixed returns a new writer that has writes with specified prefix.
func (writer *PrefixWriter) Prefixed(prefix string) WriterFlusher {
	writer.mu.Lock()
	writer.prefixlen = max(writer.prefixlen, len(prefix))
	writer.mu.Unlock()

	return &prefixWriter{
		PrefixWriter: writer,
		prefix:       prefix,
		id:           "",
		buffer:       make([]byte, 0, writer.maxline),
	}
}

// Write implements io.Writer that prefixes lines.
func (writer *PrefixWriter) Write(data []byte) (int, error) {
	return writer.root.Write(data)
}

// Flush any pending content.
func (writer *PrefixWriter) Flush() error {
	return writer.root.Flush()
}

// Write implements io.Writer that prefixes lines.
func (writer *prefixWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	writer.local.Lock()
	defer writer.local.Unlock()

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
		buffer = writer.buffer
		buffer = append(buffer, data...)
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
	timeText := writer.nowFunc().Format(timeFormat)
	for len(buffer) > 0 {
		pos := bytes.IndexByte(buffer, '\n')
		insertbreak := false

		// did not find a linebreak
		if pos < 0 {
			// wait for more data, if we haven't reached maxline
			if len(buffer) < writer.maxline {
				return len(data), nil
			}
		}

		// try to find a nice place where to break the line
		if pos < 0 || pos > writer.maxline {
			pos = writer.maxline - 1
			for p := pos; p >= writer.maxline*2/3; p-- {
				// is there a space we can break on?
				if buffer[p] == ' ' {
					pos = p
					break
				}
			}
			insertbreak = true
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
		_, err = writer.dst.Write([]byte{'\n'})
		if err != nil {
			return len(data), err
		}

		// remove the linebreak from buffer, if it's not an insert
		if !insertbreak && len(buffer) > 0 {
			buffer = buffer[1:]
		}

		prefix = ""
		id = ""
		timeText = emptyTimeField
	}

	return len(data), nil
}

// Flush flushes any pending data.
func (writer *prefixWriter) Flush() error {
	writer.local.Lock()
	defer writer.local.Unlock()

	buffer := writer.buffer
	writer.buffer = nil
	if len(buffer) == 0 {
		return nil
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()

	prefix := writer.prefix
	id := writer.id
	timeText := writer.nowFunc().Format(timeFormat)
	for len(buffer) > 0 {
		pos := bytes.IndexByte(buffer, '\n')
		insertbreak := false

		// did not find a linebreak
		if pos < 0 {
			pos = len(buffer)
		}

		// try to find a nice place where to break the line
		if pos < 0 || pos > writer.maxline {
			pos = writer.maxline - 1
			for p := pos; p >= writer.maxline*2/3; p-- {
				// is there a space we can break on?
				if buffer[p] == ' ' {
					pos = p
					break
				}
			}
			insertbreak = true
		}

		_, err := fmt.Fprintf(writer.dst, "%-*s %-*s %s | ", writer.prefixlen, prefix, maxIDLength, id, timeText)
		if err != nil {
			return err
		}

		_, err = writer.dst.Write(buffer[:pos])
		buffer = buffer[pos:]
		if err != nil {
			return err
		}
		_, err = writer.dst.Write([]byte{'\n'})
		if err != nil {
			return err
		}

		// remove the linebreak from buffer, if it's not an insert
		if !insertbreak && len(buffer) > 0 {
			buffer = buffer[1:]
		}

		prefix = ""
		id = ""
		timeText = emptyTimeField
	}

	return nil
}
