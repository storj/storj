// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build race

package tagsql

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

type tracker struct {
	parent  *tracker
	callers frames
	mu      sync.Mutex
	open    map[*tracker]struct{}
}

type frames [5]uintptr

func callers(skipCallers int) frames {
	var fs frames
	runtime.Callers(skipCallers+1, fs[:])
	return fs
}

func rootTracker(skipCallers int) *tracker {
	return &tracker{
		callers: callers(skipCallers + 1),
		open:    map[*tracker]struct{}{},
	}
}

func (t *tracker) child(skipCallers int) *tracker {
	c := rootTracker(skipCallers + 1)
	c.parent = t
	t.add(c)
	return c
}

func (t *tracker) add(r *tracker) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.open[r] = struct{}{}
}

func (t *tracker) del(r *tracker) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.open, r)
}

func (t *tracker) close() error {
	var err error
	if len(t.open) != 0 {
		var s strings.Builder
		fmt.Fprintf(&s, "--- Database created at ---\n")
		fmt.Fprintf(&s, "%s", t.callers.String())

		unique := map[frames]int{}
		for r := range t.open {
			unique[r.callers]++
		}

		for r, count := range unique {
			fmt.Fprintf(&s, "--- Unclosed resource opened from (count=%d) ---\n", count)
			fmt.Fprintf(&s, "%s", r.String())
		}

		fmt.Fprintf(&s, "--- Closing the parent of unclosed resources ---\n")
		closingFrames := callers(2)
		fmt.Fprintf(&s, "%s", closingFrames.String())
		err = errors.New(s.String())
	}
	if t.parent != nil {
		t.parent.del(t)
	}
	return err
}

func (t *tracker) formatStack() string {
	return t.callers.String()
}

func (fs *frames) String() string {
	var s strings.Builder
	frames := runtime.CallersFrames((*fs)[:])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.File, "runtime/") {
			break
		}
		fmt.Fprintf(&s, "%s\n", frame.Function)
		fmt.Fprintf(&s, "\t%s:%d\n", frame.File, frame.Line)
		if !more {
			break
		}
	}
	return s.String()
}
