// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/modular/eventkit"
	"storj.io/storj/shared/modular/profiler"
	"storj.io/storj/shared/modular/tracing"
	"storj.io/storj/shared/mud"
)

// Observability implements mud.ComponentSelectorProvider. It selects the
// standard startup modules shared by every long-running satellite service:
// observability components plus the database schema version check. It is
// intentionally not used by the migrate subcommand or pure tooling commands.
func Observability(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		mud.Select[debug.Wrapper](ball),
		mud.Select[*tracing.Tracing](ball),
		mud.Select[*eventkit.Eventkit](ball),
		mud.Select[*profiler.Profiler](ball),
		mud.Select[*DBVersionCheck](ball),
	)
}
