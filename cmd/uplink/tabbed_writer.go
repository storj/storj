// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type tabbedWriter struct {
	tw        *tabwriter.Writer
	headers   []string
	wrote     bool
	unflushed int
}

func newTabbedWriter(w io.Writer, headers ...string) *tabbedWriter {
	return &tabbedWriter{
		tw:      tabwriter.NewWriter(w, 4, 4, 4, ' ', 0),
		headers: headers,
	}
}

func (t *tabbedWriter) Done() {
	if t.wrote {
		_ = t.tw.Flush()
		t.unflushed = 0
	}
}

func (t *tabbedWriter) WriteLine(parts ...interface{}) {
	if !t.wrote {
		if len(t.headers) > 0 {
			_, _ = fmt.Fprintln(t.tw, strings.Join(t.headers, "\t"))
		}
		t.wrote = true
	}
	for i, part := range parts {
		if i > 0 {
			_, _ = fmt.Fprint(t.tw, "\t")
		}
		_, _ = fmt.Fprint(t.tw, toString(part))
	}
	_, _ = fmt.Fprintln(t.tw)
	t.unflushed++
	if t.unflushed >= 1000 {
		t.unflushed = 0
		_ = t.tw.Flush()
	}
}

func toString(x interface{}) string {
	switch x := x.(type) {
	case rune:
		return string(x)
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}
