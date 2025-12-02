// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// Package event provides a lightweight framework for tracking and reporting "main" events
// with contextual annotations throughout the application lifecycle.
//
// This approach is inspired by https://jeremymorrell.dev/blog/a-practitioners-guide-to-wide-events/
//
// It propagates event context using Go's context.Context, allowing annotations to be added later.
// Think about it as the main Trace / root Span which can be annotated any time, and will be handled by Eventkit.
//
// Unfortunately, Monkit Spans/Traces couldn't be used for this, as we couldn't reset/fork the Event context
// (preferred for batch requests, as we use dedicated Events for each batch element).
//
// Basic usage:
//
//	ctx, report := event.StartEvent(ctx, "operation-name")
//	defer report()
//	event.Annotate(ctx, "user_id", userID)
//	event.Annotate(ctx, "duration", time.Since(start))
//
// For RPC handlers:
//
//	handler := event.WrapHandler(originalHandler)
//	// All RPC calls will now be tracked with automatic event reporting
package event
