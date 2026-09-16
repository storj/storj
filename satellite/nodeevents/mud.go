// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "node-events")
	mud.View[*mailservice.Service, MailSender](ball, func(service *mailservice.Service) MailSender {
		return service
	})
	mud.Provide[Notifier](ball, NewNotifier)
}
