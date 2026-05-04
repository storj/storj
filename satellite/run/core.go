// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/satellite/accountfreeze"
	"storj.io/storj/satellite/accounting/projectbwcleanup"
	"storj.io/storj/satellite/accounting/rollup"
	"storj.io/storj/satellite/accounting/rolluparchive"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console/dbcleanup"
	"storj.io/storj/satellite/console/dbcleanup/pendingdelete"
	"storj.io/storj/satellite/console/emailreminders"
	"storj.io/storj/satellite/gc/sender"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/metabase/zombiedeletion"
	"storj.io/storj/satellite/metainfo/expireddeletion"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/overlay/offlinenodes"
	"storj.io/storj/satellite/overlay/straynodes"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/shared/mud"
)

// Core is a subcommand to start only Core services.
type Core struct {
}

// GetSelector implements mud.ComponentSelectorProvider.
func (a *Core) GetSelector(ball *mud.Ball) mud.ComponentSelector {
	return mud.Or(
		Observability(ball),
		mud.Select[*analytics.Service](ball),
		mud.Select[*mailservice.Service](ball),
		mud.Select[*emailreminders.Chore](ball),
		mud.Select[*overlay.Service](ball),
		mud.Select[*offlinenodes.Chore](ball),
		mud.Select[*straynodes.Chore](ball),
		mud.Select[*nodeevents.Chore](ball),
		mud.Select[*audit.ContainmentSyncChore](ball),
		mud.Select[*expireddeletion.Chore](ball),
		mud.Select[*zombiedeletion.Chore](ball),
		mud.Select[*tally.Service](ball),
		mud.Select[*rollup.Service](ball),
		mud.Select[*projectbwcleanup.Chore](ball),
		mud.Select[*rolluparchive.Chore](ball),
		mud.Select[*storjscan.Chore](ball),
		mud.Select[*billing.Chore](ball),
		mud.Select[*repairer.QueueStat](ball),
		mud.Select[*sender.Service](ball),
		mud.Select[*accountfreeze.Chore](ball),
		mud.Select[*accountfreeze.BotFreezeChore](ball),
		mud.Select[*accountfreeze.TrialFreezeChore](ball),
		mud.Select[*dbcleanup.Chore](ball),
		mud.Select[*pendingdelete.Chore](ball),
	)
}
