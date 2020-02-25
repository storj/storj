// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package traces

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tracetagger "github.com/jtolds/tracetagger/v2"
	monkit "github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/collect"
)

var (
	// TagDB tags a trace as hitting the database
	TagDB = tracetagger.NewTagRef()
)

// CollectTraces starts storing all known tagged traces on disk, until cancel
// is called
func CollectTraces() (cancel func()) {
	path := filepath.Join(os.TempDir(), "storj-traces", fmt.Sprint(os.Getpid()))
	disable := TagDB.Enable()
	cancelCollect := tracetagger.TracesWithTag(TagDB, 10000, func(spans []*collect.FinishedSpan, capped bool) {
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
	return func() {
		cancelCollect()
		disable()
	}
}

// Tag tags a trace with the given tag
func Tag(ctx context.Context, tag *tracetagger.TagRef) {
	tracetagger.Tag(ctx, tag)
}

// TagScope tags all functions on an entire monkit Scope with a given tag
func TagScope(tag *tracetagger.TagRef, scope *monkit.Scope) {
	tracetagger.TagScope(tag, scope)
}
