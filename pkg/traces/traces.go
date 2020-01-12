// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package traces

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jtolds/tracetagger"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"gopkg.in/spacemonkeygo/monkit.v2/collect"
)

// TraceTag is a type to keep track of different types of tags for traces
type TraceTag int

const (
	// TagDB tags a trace as hitting the database
	TagDB TraceTag = 1
)

// CollectTraces starts storing all known tagged traces on disk, until cancel
// is called
func CollectTraces() (cancel func()) {
	path := filepath.Join(os.TempDir(), "storj-traces", fmt.Sprint(os.Getpid()))
	return tracetagger.TracesWithTag(TagDB, 10000, func(spans []*collect.FinishedSpan, capped bool) {
		name := tracetagger.TracePathPrefix(spans, capped)
		err := tracetagger.SaveTrace(spans, capped, filepath.Join(path, "unfiltered", name))
		if err != nil {
			log.Print(err)
		}
		spans = tracetagger.JustTaggedSpans(spans, TagDB)
		err = tracetagger.SaveTrace(spans, capped, filepath.Join(path, "filtered", name))
		if err != nil {
			log.Print(err)
		}
	})
}

// Tag tags a trace with the given tag
func Tag(ctx context.Context, tag TraceTag) {
	tracetagger.Tag(ctx, tag)
}

// TagScope tags all functions on an entire monkit Scope with a given tag
func TagScope(tag TraceTag, scope *monkit.Scope) {
	tracetagger.TagScope(tag, scope)
}
