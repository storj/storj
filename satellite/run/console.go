// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/private/healthcheck"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/shared/mud"
)

// Console defines the satellite console configuration and component selection.
type Console struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Console) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*consoleweb.Server](ball),
		mud.Select[*healthcheck.Server](ball))
}
