// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

type PrefixWriter struct {
	root    *prefixWriter
	maxline int

	mu  sync.Mutex
	len int
	dst io.Writer
}

func NewPrefixWriter(prefix string, dst io.Writer) *PrefixWriter {
	writer := &PrefixWriter{
		maxline: 10000, // disable maxline cutting
		dst:     dst,
	}
	writer.root = writer.Prefixed(prefix).(*prefixWriter)
	return writer
}

type prefixWriter struct {
	*PrefixWriter
	prefix string
	buffer []byte
}

func (writer *PrefixWriter) Prefixed(prefix string) io.Writer {
	writer.mu.Lock()
	if len(prefix) > writer.len {
		writer.len = len(prefix)
	}
	writer.mu.Unlock()

	return &prefixWriter{writer, prefix, make([]byte, 0, writer.maxline)}
}

func (writer *PrefixWriter) Write(data []byte) (int, error) {
	return writer.root.Write(data)
}

func (writer *prefixWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
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

	prefix := writer.prefix
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

		_, err := fmt.Fprintf(writer.dst, "%-*s | ", writer.len, prefix)
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
	}

	return len(data), nil
}
